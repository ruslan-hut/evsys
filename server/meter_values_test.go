package server

import (
	"testing"
	"time"

	"evsys/entity"
	"evsys/internal"
	"evsys/ocpp/v16/core"
	"evsys/types"
)

// meterStubDB records the meter values the handler stores. The embedded
// interface is nil: any method the handler calls that is not implemented here
// panics, which keeps the stub honest about what OnMeterValues depends on.
type meterStubDB struct {
	internal.Database
	transaction *entity.Transaction
	stored      []*entity.TransactionMeter
}

func (s *meterStubDB) GetTransaction(_ int) (*entity.Transaction, error) {
	return s.transaction, nil
}

func (s *meterStubDB) AddTransactionMeterValue(m *entity.TransactionMeter) error {
	s.stored = append(s.stored, m)
	return nil
}

func (s *meterStubDB) GetConnector(_ int, _ string) (*entity.Connector, error) {
	return nil, nil
}

func (s *meterStubDB) AddConnector(*entity.Connector) error { return nil }

type meterStubBilling struct{}

func (meterStubBilling) OnTransactionStart(*entity.Transaction) error    { return nil }
func (meterStubBilling) OnTransactionFinished(*entity.Transaction) error { return nil }
func (meterStubBilling) OnMeterValue(*entity.Transaction, *entity.TransactionMeter) error {
	return nil
}

type meterStubLogger struct{}

func (meterStubLogger) FeatureEvent(_, _, _ string) {}
func (meterStubLogger) RawDataEvent(_, _ string)    {}
func (meterStubLogger) Debug(_ string)              {}
func (meterStubLogger) Warn(_ string)               {}
func (meterStubLogger) Error(_ string, _ error)     {}

// TestOnMeterValuesStoresEveryContext guards against the regression where a
// charge point reporting meter values on its own had them silently discarded:
// readings were only kept when their context was Trigger, so a charger using
// MeterValueSampleInterval produced transactions with no meter values at all.
func TestOnMeterValuesStoresEveryContext(t *testing.T) {
	contexts := []types.ReadingContext{
		types.ReadingContextSamplePeriodic,
		types.ReadingContextTrigger,
		types.ReadingContextSampleClock,
		types.ReadingContextOther,
	}

	for _, readingContext := range contexts {
		for _, triggerEnabled := range []bool{true, false} {
			t.Run(string(readingContext), func(t *testing.T) {
				db := &meterStubDB{
					transaction: &entity.Transaction{Id: 1, MeterStart: 1000},
				}
				h := &SystemHandler{
					chargePoints: map[string]*ChargePointState{},
					lastMeter:    map[int]*entity.TransactionMeter{},
					database:     db,
					billing:      meterStubBilling{},
					logger:       meterStubLogger{},
				}
				state := newChargePointState(&entity.ChargePoint{
					Id:             "CP1",
					TriggerMessage: triggerEnabled,
				})
				h.chargePoints["CP1"] = state

				transactionId := 1
				request := &core.MeterValuesRequest{
					ConnectorId:   1,
					TransactionId: &transactionId,
					MeterValue: []types.MeterValue{{
						Timestamp: types.NewDateTime(time.Now()),
						SampledValue: []types.SampledValue{{
							Value:     "1500",
							Context:   readingContext,
							Measurand: types.MeasurandEnergyActiveImportRegister,
							Unit:      types.UnitOfMeasureWh,
						}},
					}},
				}

				if _, err := h.OnMeterValues("CP1", request); err != nil {
					t.Fatalf("OnMeterValues: %v", err)
				}

				if len(db.stored) != 1 {
					t.Fatalf("context %s, trigger_message %v: stored %d meter values, want 1",
						readingContext, triggerEnabled, len(db.stored))
				}
				if got := db.stored[0].Value; got != 1500 {
					t.Errorf("stored value = %d, want 1500", got)
				}
				if got := db.stored[0].ConsumedEnergy; got != 500 {
					t.Errorf("consumed energy = %d, want 500", got)
				}
			})
		}
	}
}

// TestOnMeterValuesRecoversTransactionFromConnector guards the fallback for
// chargers that report meter values on their own (e.g. trigger_message
// disabled) and omit the optional transactionId: the running transaction is
// recovered from the connector so the readings are still stored instead of
// dropped. Without the fallback these transactions end up with no meter values.
func TestOnMeterValuesRecoversTransactionFromConnector(t *testing.T) {
	db := &meterStubDB{
		transaction: &entity.Transaction{Id: 7, MeterStart: 1000},
	}
	h := &SystemHandler{
		chargePoints: map[string]*ChargePointState{},
		lastMeter:    map[int]*entity.TransactionMeter{},
		database:     db,
		billing:      meterStubBilling{},
		logger:       meterStubLogger{},
	}
	state := newChargePointState(&entity.ChargePoint{
		Id:             "CP1",
		TriggerMessage: false,
	})
	connector := entity.NewConnector(1, "CP1")
	connector.CurrentTransactionId = 7
	state.connectors[1] = connector
	h.chargePoints["CP1"] = state

	request := &core.MeterValuesRequest{
		ConnectorId:   1,
		TransactionId: nil, // charger omits it
		MeterValue: []types.MeterValue{{
			Timestamp: types.NewDateTime(time.Now()),
			SampledValue: []types.SampledValue{{
				Value:     "1500",
				Context:   types.ReadingContextSamplePeriodic,
				Measurand: types.MeasurandEnergyActiveImportRegister,
				Unit:      types.UnitOfMeasureWh,
			}},
		}},
	}

	if _, err := h.OnMeterValues("CP1", request); err != nil {
		t.Fatalf("OnMeterValues: %v", err)
	}

	if len(db.stored) != 1 {
		t.Fatalf("stored %d meter values, want 1", len(db.stored))
	}
	if got := db.stored[0].Id; got != 7 {
		t.Errorf("stored transaction id = %d, want 7", got)
	}
	if got := db.stored[0].ConsumedEnergy; got != 500 {
		t.Errorf("consumed energy = %d, want 500", got)
	}
}

// TestOnMeterValuesDropsWithoutActiveTransaction confirms the fallback does not
// invent a transaction: with no transactionId and an idle connector the reading
// is discarded rather than attributed to a stale or absent transaction.
func TestOnMeterValuesDropsWithoutActiveTransaction(t *testing.T) {
	db := &meterStubDB{}
	h := &SystemHandler{
		chargePoints: map[string]*ChargePointState{},
		lastMeter:    map[int]*entity.TransactionMeter{},
		database:     db,
		billing:      meterStubBilling{},
		logger:       meterStubLogger{},
		trigger:      &Trigger{Unregister: make(chan int, 1)},
	}
	state := newChargePointState(&entity.ChargePoint{Id: "CP1"})
	state.connectors[1] = entity.NewConnector(1, "CP1") // CurrentTransactionId -1
	h.chargePoints["CP1"] = state

	request := &core.MeterValuesRequest{
		ConnectorId:   1,
		TransactionId: nil,
		MeterValue: []types.MeterValue{{
			Timestamp: types.NewDateTime(time.Now()),
			SampledValue: []types.SampledValue{{
				Value:     "1500",
				Context:   types.ReadingContextSamplePeriodic,
				Measurand: types.MeasurandEnergyActiveImportRegister,
				Unit:      types.UnitOfMeasureWh,
			}},
		}},
	}

	if _, err := h.OnMeterValues("CP1", request); err != nil {
		t.Fatalf("OnMeterValues: %v", err)
	}
	if len(db.stored) != 0 {
		t.Fatalf("stored %d meter values, want 0", len(db.stored))
	}
}
