package server

import (
	"evsys/entity"
	"evsys/ocpp/common"
	"evsys/ocpp/v201"
	"fmt"
	"time"
)

// ProtocolAdapter provides abstraction layer for converting between
// different OCPP protocol versions and internal business entities.
// This allows the business logic to remain version-agnostic.
type ProtocolAdapter struct{}

func NewProtocolAdapter() *ProtocolAdapter {
	return &ProtocolAdapter{}
}

// ============================================================================
// AUTHORIZATION ADAPTERS
// ============================================================================

// IdToken201ToIdTag converts OCPP 2.0.1 IdToken to OCPP 1.6J IdTag string
func (pa *ProtocolAdapter) IdToken201ToIdTag(token *v201.IdToken) string {
	if token == nil {
		return ""
	}
	return token.IdToken
}

// IdTagToIdToken201 converts OCPP 1.6J IdTag string to OCPP 2.0.1 IdToken
func (pa *ProtocolAdapter) IdTagToIdToken201(idTag string) *v201.IdToken {
	if idTag == "" {
		return nil
	}
	return &v201.IdToken{
		IdToken: idTag,
		Type:    v201.IdTokenTypeISO14443, // Default to RFID
	}
}

// AuthStatusToV16Status converts v201 authorization status to v16 status string
func (pa *ProtocolAdapter) AuthStatusToV16Status(status v201.AuthorizationStatusType) string {
	switch status {
	case v201.AuthorizationStatusAccepted:
		return "Accepted"
	case v201.AuthorizationStatusBlocked:
		return "Blocked"
	case v201.AuthorizationStatusExpired:
		return "Expired"
	case v201.AuthorizationStatusInvalid:
		return "Invalid"
	case v201.AuthorizationStatusUnknown:
		return "Invalid"
	default:
		return "Invalid"
	}
}

// V16StatusToAuthStatus converts v16 status string to v201 authorization status
func (pa *ProtocolAdapter) V16StatusToAuthStatus(status string) v201.AuthorizationStatusType {
	switch status {
	case "Accepted":
		return v201.AuthorizationStatusAccepted
	case "Blocked":
		return v201.AuthorizationStatusBlocked
	case "Expired":
		return v201.AuthorizationStatusExpired
	case "Invalid":
		return v201.AuthorizationStatusInvalid
	default:
		return v201.AuthorizationStatusInvalid
	}
}

// ============================================================================
// TRANSACTION ADAPTERS
// ============================================================================

// TransactionEventToEntity converts OCPP 2.0.1 TransactionEvent to internal Transaction entity
// This handles the mapping of the unified TransactionEvent message to the internal transaction model.
func (pa *ProtocolAdapter) TransactionEventToEntity(
	eventType v201.TransactionEventType,
	transactionInfo v201.Transaction,
	idToken *v201.IdToken,
	evse *v201.EVSE,
	meterValues []v201.MeterValue,
	timestamp time.Time,
	chargePointId string,
) (*entity.Transaction, error) {

	tx := &entity.Transaction{
		SessionId:       transactionInfo.TransactionId,
		ChargePointId:   chargePointId,
		ProtocolVersion: string(common.OCPP201),
		TimeStart:       timestamp,
		IsFinished:      false,
	}

	// Map IdToken to IdTag
	if idToken != nil {
		tx.IdTag = idToken.IdToken
	}

	// Map EVSE information
	if evse != nil {
		tx.EvseId = &evse.Id
		if evse.ConnectorId != nil {
			tx.ConnectorId = *evse.ConnectorId
		}
	}

	// Map reason if transaction ended
	if eventType == v201.TransactionEventEnded {
		tx.IsFinished = true
		tx.TimeStop = timestamp
		if transactionInfo.StoppedReason != "" {
			tx.Reason = string(transactionInfo.StoppedReason)
		}
	}

	// Map meter values
	if len(meterValues) > 0 {
		// Get the latest meter value for transaction start/stop
		latestMeter := meterValues[len(meterValues)-1]
		for _, sampledValue := range latestMeter.SampledValue {
			// Look for Energy.Active.Import.Register (equivalent to OCPP 1.6 Energy.Active.Import.Register)
			if sampledValue.Measurand == v201.MeasurandEnergyActiveImportRegister ||
				sampledValue.Measurand == "" { // Default measurand
				meterValueWh := int(sampledValue.Value)

				if eventType == v201.TransactionEventStarted {
					tx.MeterStart = meterValueWh
				} else if eventType == v201.TransactionEventEnded {
					tx.MeterStop = meterValueWh
				}
				break
			}
		}
	}

	// Store original transaction info in metadata for reference
	if tx.Metadata == nil {
		tx.Metadata = make(map[string]interface{})
	}
	tx.Metadata["ocpp201_transaction_id"] = transactionInfo.TransactionId
	if transactionInfo.ChargingState != "" {
		tx.Metadata["charging_state"] = string(transactionInfo.ChargingState)
	}
	if transactionInfo.RemoteStartId != nil {
		tx.Metadata["remote_start_id"] = *transactionInfo.RemoteStartId
	}

	return tx, nil
}

// EntityToTransactionInfo converts internal Transaction entity to OCPP 2.0.1 Transaction info
func (pa *ProtocolAdapter) EntityToTransactionInfo(tx *entity.Transaction) v201.Transaction {
	transactionInfo := v201.Transaction{
		TransactionId: tx.SessionId,
	}

	// Map charging state from metadata if available
	if tx.Metadata != nil {
		if chargingState, ok := tx.Metadata["charging_state"].(string); ok {
			transactionInfo.ChargingState = v201.ChargingStateType(chargingState)
		}
		if remoteStartId, ok := tx.Metadata["remote_start_id"].(int); ok {
			transactionInfo.RemoteStartId = &remoteStartId
		}
	}

	// Map stopped reason
	if tx.IsFinished && tx.Reason != "" {
		transactionInfo.StoppedReason = v201.ReasonType(tx.Reason)
	}

	return transactionInfo
}

// ============================================================================
// METER VALUE ADAPTERS
// ============================================================================

// MeterValue201ToTransactionMeter converts OCPP 2.0.1 MeterValue to internal TransactionMeter
func (pa *ProtocolAdapter) MeterValue201ToTransactionMeter(
	meterValue v201.MeterValue,
	transactionId int,
) (*entity.TransactionMeter, error) {

	tm := &entity.TransactionMeter{
		Id:   transactionId,
		Time: meterValue.Timestamp,
	}

	// Extract energy value from sampled values
	for _, sampledValue := range meterValue.SampledValue {
		// Default measurand is Energy.Active.Import.Register
		measurand := sampledValue.Measurand
		if measurand == "" {
			measurand = v201.MeasurandEnergyActiveImportRegister
		}

		switch measurand {
		case v201.MeasurandEnergyActiveImportRegister:
			tm.Value = int(sampledValue.Value)
			tm.Measurand = "Energy.Active.Import.Register"
			tm.Unit = "Wh"
		case v201.MeasurandPowerActiveImport:
			tm.PowerActive = int(sampledValue.Value)
		case v201.MeasurandSoC:
			tm.BatteryLevel = int(sampledValue.Value)
		}

		// Handle unit of measure multiplier
		if sampledValue.UnitOfMeasure != nil && sampledValue.UnitOfMeasure.Multiplier != nil {
			multiplier := *sampledValue.UnitOfMeasure.Multiplier
			// Apply multiplier (10^multiplier)
			switch multiplier {
			case -3: // milli
				tm.Value = tm.Value / 1000
			case -2: // centi
				tm.Value = tm.Value / 100
			case -1: // deci
				tm.Value = tm.Value / 10
			case 1: // deca
				tm.Value = tm.Value * 10
			case 2: // hecto
				tm.Value = tm.Value * 100
			case 3: // kilo
				tm.Value = tm.Value * 1000
			}
		}
	}

	return tm, nil
}

// TransactionMeterToMeterValue201 converts internal TransactionMeter to OCPP 2.0.1 MeterValue
func (pa *ProtocolAdapter) TransactionMeterToMeterValue201(tm *entity.TransactionMeter) v201.MeterValue {
	sampledValues := []v201.SampledValue{}

	// Add energy value
	if tm.Value > 0 {
		sampledValues = append(sampledValues, v201.SampledValue{
			Value:     float64(tm.Value),
			Context:   v201.ReadingContextSamplePeriodic,
			Measurand: v201.MeasurandEnergyActiveImportRegister,
			UnitOfMeasure: &v201.UnitOfMeasure{
				Unit: v201.UnitWh,
			},
		})
	}

	// Add power value
	if tm.PowerActive > 0 {
		sampledValues = append(sampledValues, v201.SampledValue{
			Value:     float64(tm.PowerActive),
			Measurand: v201.MeasurandPowerActiveImport,
			UnitOfMeasure: &v201.UnitOfMeasure{
				Unit: v201.UnitW,
			},
		})
	}

	// Add battery level (SoC) if available
	if tm.BatteryLevel > 0 {
		sampledValues = append(sampledValues, v201.SampledValue{
			Value:     float64(tm.BatteryLevel),
			Measurand: v201.MeasurandSoC,
			UnitOfMeasure: &v201.UnitOfMeasure{
				Unit: v201.UnitPercent,
			},
		})
	}

	return v201.MeterValue{
		Timestamp:    tm.Time,
		SampledValue: sampledValues,
	}
}

// ============================================================================
// EVSE ADAPTERS
// ============================================================================

// EvseToConnectorId extracts connector ID from OCPP 2.0.1 EVSE
// In OCPP 2.0.1, the hierarchy is ChargingStation → EVSE → Connector
// For backwards compatibility, we map EVSE ID to connector ID when needed
func (pa *ProtocolAdapter) EvseToConnectorId(evse *v201.EVSE) int {
	if evse == nil {
		return 0
	}
	if evse.ConnectorId != nil {
		return *evse.ConnectorId
	}
	// If no connector ID specified, use EVSE ID as connector ID
	return evse.Id
}

// ConnectorIdToEvse creates OCPP 2.0.1 EVSE from connector ID
// For backwards compatibility with OCPP 1.6J
func (pa *ProtocolAdapter) ConnectorIdToEvse(connectorId int, evseId *int) *v201.EVSE {
	evse := &v201.EVSE{
		ConnectorId: &connectorId,
	}

	if evseId != nil {
		evse.Id = *evseId
	} else {
		// Default: use connector ID as EVSE ID for 1.6 compatibility
		evse.Id = connectorId
	}

	return evse
}

// ============================================================================
// VALIDATION HELPERS
// ============================================================================

// ValidateProtocolVersion checks if the protocol version is supported
func (pa *ProtocolAdapter) ValidateProtocolVersion(version string) error {
	switch version {
	case string(common.OCPP16), string(common.OCPP201), string(common.OCPP21):
		return nil
	default:
		return fmt.Errorf("unsupported protocol version: %s", version)
	}
}

// IsOCPP201OrHigher checks if the protocol version is OCPP 2.0.1 or higher
func (pa *ProtocolAdapter) IsOCPP201OrHigher(version string) bool {
	return version == string(common.OCPP201) || version == string(common.OCPP21)
}

// IsOCPP16 checks if the protocol version is OCPP 1.6J
func (pa *ProtocolAdapter) IsOCPP16(version string) bool {
	return version == string(common.OCPP16) || version == ""
}
