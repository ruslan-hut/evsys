package server

import (
	"evsys/ocpp/v201"
	"testing"
	"time"
)

// ============================================================================
// Protocol Adapter Tests
// ============================================================================
// Tests for converting between OCPP 1.6J and 2.0.1 types and internal entities
// ============================================================================

func TestProtocolAdapter_IdToken201ToIdTag(t *testing.T) {
	adapter := NewProtocolAdapter()

	tests := []struct {
		name     string
		token    *v201.IdToken
		expected string
	}{
		{
			name: "ISO14443 token",
			token: &v201.IdToken{
				IdToken: "AABBCCDD",
				Type:    v201.IdTokenTypeISO14443,
			},
			expected: "AABBCCDD",
		},
		{
			name: "Central token",
			token: &v201.IdToken{
				IdToken: "user123",
				Type:    v201.IdTokenTypeCentral,
			},
			expected: "user123",
		},
		{
			name: "KeyCode token",
			token: &v201.IdToken{
				IdToken: "1234",
				Type:    v201.IdTokenTypeKeyCode,
			},
			expected: "1234",
		},
		{
			name:     "Nil token",
			token:    nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.IdToken201ToIdTag(tt.token)
			if result != tt.expected {
				t.Errorf("IdToken201ToIdTag() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestProtocolAdapter_IdTagToIdToken201(t *testing.T) {
	adapter := NewProtocolAdapter()

	tests := []struct {
		name     string
		idTag    string
		expected *v201.IdToken
	}{
		{
			name:  "Simple ID tag",
			idTag: "AABBCCDD",
			expected: &v201.IdToken{
				IdToken: "AABBCCDD",
				Type:    v201.IdTokenTypeISO14443,
			},
		},
		{
			name:     "Empty ID tag",
			idTag:    "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.IdTagToIdToken201(tt.idTag)
			if tt.expected == nil {
				if result != nil {
					t.Errorf("IdTagToIdToken201() = %v, want nil", result)
				}
			} else if result == nil || result.IdToken != tt.expected.IdToken || result.Type != tt.expected.Type {
				t.Errorf("IdTagToIdToken201() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestProtocolAdapter_EvseToConnectorId(t *testing.T) {
	adapter := NewProtocolAdapter()

	tests := []struct {
		name     string
		evse     *v201.EVSE
		expected int
	}{
		{
			name: "EVSE with connector 1",
			evse: &v201.EVSE{
				Id:          1,
				ConnectorId: intPtr(1),
			},
			expected: 1,
		},
		{
			name: "EVSE with connector 2",
			evse: &v201.EVSE{
				Id:          2,
				ConnectorId: intPtr(2),
			},
			expected: 2,
		},
		{
			name: "EVSE without connector ID",
			evse: &v201.EVSE{
				Id:          1,
				ConnectorId: nil,
			},
			expected: 1, // Returns EVSE ID as fallback
		},
		{
			name:     "Nil EVSE",
			evse:     nil,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.EvseToConnectorId(tt.evse)
			if result != tt.expected {
				t.Errorf("EvseToConnectorId() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestProtocolAdapter_ConnectorIdToEvse(t *testing.T) {
	adapter := NewProtocolAdapter()

	tests := []struct {
		name        string
		connectorId int
		evseId      *int
		expected    *v201.EVSE
	}{
		{
			name:        "Connector 1 with EVSE 1",
			connectorId: 1,
			evseId:      intPtr(1),
			expected: &v201.EVSE{
				Id:          1,
				ConnectorId: intPtr(1),
			},
		},
		{
			name:        "Connector 2 without EVSE",
			connectorId: 2,
			evseId:      nil,
			expected: &v201.EVSE{
				Id:          2,
				ConnectorId: intPtr(2),
			},
		},
		{
			name:        "Connector 0 (charging station)",
			connectorId: 0,
			evseId:      intPtr(0),
			expected: &v201.EVSE{
				Id:          0,
				ConnectorId: intPtr(0),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.ConnectorIdToEvse(tt.connectorId, tt.evseId)
			if result.Id != tt.expected.Id {
				t.Errorf("ConnectorIdToEvse() Id = %v, want %v", result.Id, tt.expected.Id)
			}
			if result.ConnectorId == nil || tt.expected.ConnectorId == nil {
				if result.ConnectorId != tt.expected.ConnectorId {
					t.Errorf("ConnectorIdToEvse() ConnectorId = %v, want %v", result.ConnectorId, tt.expected.ConnectorId)
				}
			} else if *result.ConnectorId != *tt.expected.ConnectorId {
				t.Errorf("ConnectorIdToEvse() ConnectorId = %v, want %v", *result.ConnectorId, *tt.expected.ConnectorId)
			}
		})
	}
}

func TestProtocolAdapter_TransactionEventToEntity_Started(t *testing.T) {
	adapter := NewProtocolAdapter()

	now := time.Now()
	idToken := &v201.IdToken{
		IdToken: "TESTTOKEN",
		Type:    v201.IdTokenTypeISO14443,
	}
	evse := &v201.EVSE{
		Id:          1,
		ConnectorId: intPtr(1),
	}
	transactionInfo := v201.Transaction{
		TransactionId: "TX123",
	}

	transaction, err := adapter.TransactionEventToEntity(
		v201.TransactionEventStarted,
		transactionInfo,
		idToken,
		evse,
		nil,
		now,
		"CP001",
	)

	if err != nil {
		t.Fatalf("TransactionEventToEntity() error = %v", err)
	}

	if transaction == nil {
		t.Fatal("TransactionEventToEntity() returned nil transaction")
	}

	// Verify basic fields
	if transaction.ChargePointId != "CP001" {
		t.Errorf("ChargePointId = %v, want CP001", transaction.ChargePointId)
	}
	if transaction.ConnectorId != 1 {
		t.Errorf("ConnectorId = %v, want 1", transaction.ConnectorId)
	}
	if transaction.IdTag != "TESTTOKEN" {
		t.Errorf("IdTag = %v, want TESTTOKEN", transaction.IdTag)
	}
	if !transaction.TimeStart.Equal(now) {
		t.Errorf("TimeStart = %v, want %v", transaction.TimeStart, now)
	}
	if transaction.EvseId == nil || *transaction.EvseId != 1 {
		t.Errorf("EvseId = %v, want 1", transaction.EvseId)
	}
}

func TestProtocolAdapter_TransactionEventToEntity_Ended(t *testing.T) {
	adapter := NewProtocolAdapter()

	now := time.Now()
	idToken := &v201.IdToken{
		IdToken: "TESTTOKEN",
		Type:    v201.IdTokenTypeISO14443,
	}
	evse := &v201.EVSE{
		Id:          1,
		ConnectorId: intPtr(1),
	}
	transactionInfo := v201.Transaction{
		TransactionId: "TX123",
		ChargingState: v201.ChargingStateSuspendedEV,
		StoppedReason: v201.ReasonStoppedByEV,
	}

	// Create meter value for stop reading
	meterValues := []v201.MeterValue{
		{
			Timestamp: now,
			SampledValue: []v201.SampledValue{
				{
					Value:     15000.0,
					Context:   v201.ReadingContextTransactionEnd,
					Measurand: v201.MeasurandEnergyActiveImportRegister,
					UnitOfMeasure: &v201.UnitOfMeasure{
						Unit: "Wh",
					},
				},
			},
		},
	}

	transaction, err := adapter.TransactionEventToEntity(
		v201.TransactionEventEnded,
		transactionInfo,
		idToken,
		evse,
		meterValues,
		now,
		"CP001",
	)

	if err != nil {
		t.Fatalf("TransactionEventToEntity() error = %v", err)
	}

	// Verify stop fields
	if !transaction.TimeStop.Equal(now) {
		t.Errorf("TimeStop = %v, want %v", transaction.TimeStop, now)
	}
	if transaction.MeterStop != 15000 {
		t.Errorf("MeterStop = %v, want 15000", transaction.MeterStop)
	}
	if transaction.Reason != string(v201.ReasonStoppedByEV) {
		t.Errorf("Reason = %v, want %v", transaction.Reason, v201.ReasonStoppedByEV)
	}
}

func TestProtocolAdapter_MeterValue201ToTransactionMeter(t *testing.T) {
	adapter := NewProtocolAdapter()

	now := time.Now()
	meterValue := v201.MeterValue{
		Timestamp: now,
		SampledValue: []v201.SampledValue{
			{
				Value:     12345.0,
				Context:   v201.ReadingContextSamplePeriodic,
				Measurand: v201.MeasurandEnergyActiveImportRegister,
				UnitOfMeasure: &v201.UnitOfMeasure{
					Unit: "Wh",
				},
			},
		},
	}

	tm, err := adapter.MeterValue201ToTransactionMeter(meterValue, 100)
	if err != nil {
		t.Fatalf("MeterValue201ToTransactionMeter() error = %v", err)
	}

	if tm.Id != 100 {
		t.Errorf("Id = %v, want 100", tm.Id)
	}
	if tm.Value != 12345 {
		t.Errorf("Value = %v, want 12345", tm.Value)
	}
	if tm.Measurand != "Energy.Active.Import.Register" {
		t.Errorf("Measurand = %v, want Energy.Active.Import.Register", tm.Measurand)
	}
	if tm.Unit != "Wh" {
		t.Errorf("Unit = %v, want Wh", tm.Unit)
	}
	if !tm.Time.Equal(now) {
		t.Errorf("Time = %v, want %v", tm.Time, now)
	}
}

func TestProtocolAdapter_MeterValue201ToTransactionMeter_PowerActive(t *testing.T) {
	adapter := NewProtocolAdapter()

	now := time.Now()
	meterValue := v201.MeterValue{
		Timestamp: now,
		SampledValue: []v201.SampledValue{
			{
				Value:     5000.0,
				Context:   v201.ReadingContextSamplePeriodic,
				Measurand: v201.MeasurandPowerActiveImport,
				UnitOfMeasure: &v201.UnitOfMeasure{
					Unit: "W",
				},
			},
		},
	}

	tm, err := adapter.MeterValue201ToTransactionMeter(meterValue, 100)
	if err != nil {
		t.Fatalf("MeterValue201ToTransactionMeter() error = %v", err)
	}

	if tm.PowerActive != 5000 {
		t.Errorf("PowerActive = %v, want 5000", tm.PowerActive)
	}
}

func TestProtocolAdapter_MeterValue201ToTransactionMeter_SoC(t *testing.T) {
	adapter := NewProtocolAdapter()

	now := time.Now()
	meterValue := v201.MeterValue{
		Timestamp: now,
		SampledValue: []v201.SampledValue{
			{
				Value:     75.0,
				Context:   v201.ReadingContextSamplePeriodic,
				Measurand: v201.MeasurandSoC,
				UnitOfMeasure: &v201.UnitOfMeasure{
					Unit: "Percent",
				},
			},
		},
	}

	tm, err := adapter.MeterValue201ToTransactionMeter(meterValue, 100)
	if err != nil {
		t.Fatalf("MeterValue201ToTransactionMeter() error = %v", err)
	}

	if tm.BatteryLevel != 75 {
		t.Errorf("BatteryLevel = %v, want 75", tm.BatteryLevel)
	}
}

// Helper function to create int pointer
func intPtr(i int) *int {
	return &i
}
