package v201

import (
	"encoding/json"
	"testing"
	"time"
)

// ============================================================================
// OCPP 2.0.1 Types Tests
// ============================================================================
// Tests for OCPP 2.0.1 type definitions, serialization, and validation
// ============================================================================

func TestIdToken_Serialization(t *testing.T) {
	token := IdToken{
		IdToken: "AABBCCDD",
		Type:    IdTokenTypeISO14443,
	}

	// Test marshaling
	data, err := json.Marshal(token)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Test unmarshaling
	var decoded IdToken
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.IdToken != token.IdToken {
		t.Errorf("IdToken = %v, want %v", decoded.IdToken, token.IdToken)
	}
	if decoded.Type != token.Type {
		t.Errorf("Type = %v, want %v", decoded.Type, token.Type)
	}
}

func TestEVSE_Serialization(t *testing.T) {
	connectorId := 1
	evse := EVSE{
		Id:          1,
		ConnectorId: &connectorId,
	}

	data, err := json.Marshal(evse)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded EVSE
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.Id != evse.Id {
		t.Errorf("Id = %v, want %v", decoded.Id, evse.Id)
	}
	if decoded.ConnectorId == nil || *decoded.ConnectorId != *evse.ConnectorId {
		t.Errorf("ConnectorId = %v, want %v", decoded.ConnectorId, evse.ConnectorId)
	}
}

func TestIdTokenInfo_AllStatuses(t *testing.T) {
	statuses := []AuthorizationStatusType{
		AuthorizationStatusAccepted,
		AuthorizationStatusBlocked,
		AuthorizationStatusConcurrentTx,
		AuthorizationStatusExpired,
		AuthorizationStatusInvalid,
		AuthorizationStatusNoCredit,
		AuthorizationStatusNotAllowedTypeEVSE,
		AuthorizationStatusNotAtThisLocation,
		AuthorizationStatusNotAtThisTime,
		AuthorizationStatusUnknown,
	}

	for _, status := range statuses {
		info := IdTokenInfo{
			Status: status,
		}

		data, err := json.Marshal(info)
		if err != nil {
			t.Errorf("json.Marshal() error for status %v = %v", status, err)
			continue
		}

		var decoded IdTokenInfo
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			t.Errorf("json.Unmarshal() error for status %v = %v", status, err)
			continue
		}

		if decoded.Status != status {
			t.Errorf("Status = %v, want %v", decoded.Status, status)
		}
	}
}

func TestTransaction_Serialization(t *testing.T) {
	tx := Transaction{
		TransactionId:     "TX12345",
		ChargingState:     ChargingStateCharging,
		TimeSpentCharging: intPtr(3600),
		RemoteStartId:     intPtr(100),
	}

	data, err := json.Marshal(tx)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded Transaction
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.TransactionId != tx.TransactionId {
		t.Errorf("TransactionId = %v, want %v", decoded.TransactionId, tx.TransactionId)
	}
	if decoded.ChargingState != tx.ChargingState {
		t.Errorf("ChargingState = %v, want %v", decoded.ChargingState, tx.ChargingState)
	}
}

func TestChargingStation_Serialization(t *testing.T) {
	station := ChargingStation{
		Model:           "Model X",
		VendorName:      "Vendor Inc",
		SerialNumber:    "SN123456",
		FirmwareVersion: "1.0.0",
		Modem: &Modem{
			Iccid: "123456789",
			Imsi:  "987654321",
		},
	}

	data, err := json.Marshal(station)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded ChargingStation
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.Model != station.Model {
		t.Errorf("Model = %v, want %v", decoded.Model, station.Model)
	}
	if decoded.VendorName != station.VendorName {
		t.Errorf("VendorName = %v, want %v", decoded.VendorName, station.VendorName)
	}
	if decoded.Modem == nil || decoded.Modem.Iccid != station.Modem.Iccid {
		t.Errorf("Modem.Iccid = %v, want %v", decoded.Modem, station.Modem)
	}
}

func TestMeterValue_Serialization(t *testing.T) {
	now := time.Now()
	mv := MeterValue{
		Timestamp: now,
		SampledValue: []SampledValue{
			{
				Value:     12345.0,
				Context:   ReadingContextSamplePeriodic,
				Measurand: MeasurandEnergyActiveImportRegister,
				Phase:     PhaseL1,
				Location:  LocationOutlet,
				UnitOfMeasure: &UnitOfMeasure{
					Unit:       "Wh",
					Multiplier: intPtr(0),
				},
			},
		},
	}

	data, err := json.Marshal(mv)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded MeterValue
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(decoded.SampledValue) != 1 {
		t.Fatalf("SampledValue length = %v, want 1", len(decoded.SampledValue))
	}

	sv := decoded.SampledValue[0]
	if sv.Value != 12345.0 {
		t.Errorf("Value = %v, want 12345.0", sv.Value)
	}
	if sv.Measurand != MeasurandEnergyActiveImportRegister {
		t.Errorf("Measurand = %v, want %v", sv.Measurand, MeasurandEnergyActiveImportRegister)
	}
	if sv.UnitOfMeasure == nil || sv.UnitOfMeasure.Unit != "Wh" {
		t.Errorf("UnitOfMeasure.Unit = %v, want Wh", sv.UnitOfMeasure)
	}
}

func TestConnectorStatusType_AllValues(t *testing.T) {
	statuses := []ConnectorStatusType{
		ConnectorStatusAvailable,
		ConnectorStatusOccupied,
		ConnectorStatusReserved,
		ConnectorStatusUnavailable,
		ConnectorStatusFaulted,
	}

	for _, status := range statuses {
		data, err := json.Marshal(status)
		if err != nil {
			t.Errorf("json.Marshal() error for %v = %v", status, err)
			continue
		}

		var decoded ConnectorStatusType
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			t.Errorf("json.Unmarshal() error for %v = %v", status, err)
			continue
		}

		if decoded != status {
			t.Errorf("Status = %v, want %v", decoded, status)
		}
	}
}

func TestTransactionEventType_AllValues(t *testing.T) {
	events := []TransactionEventType{
		TransactionEventStarted,
		TransactionEventUpdated,
		TransactionEventEnded,
	}

	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			t.Errorf("json.Marshal() error for %v = %v", event, err)
			continue
		}

		var decoded TransactionEventType
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			t.Errorf("json.Unmarshal() error for %v = %v", event, err)
			continue
		}

		if decoded != event {
			t.Errorf("Event = %v, want %v", decoded, event)
		}
	}
}

func TestChargingStateType_AllValues(t *testing.T) {
	states := []ChargingStateType{
		ChargingStateCharging,
		ChargingStateEVConnected,
		ChargingStateSuspendedEV,
		ChargingStateSuspendedEVSE,
		ChargingStateIdle,
	}

	for _, state := range states {
		data, err := json.Marshal(state)
		if err != nil {
			t.Errorf("json.Marshal() error for %v = %v", state, err)
			continue
		}

		var decoded ChargingStateType
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			t.Errorf("json.Unmarshal() error for %v = %v", state, err)
			continue
		}

		if decoded != state {
			t.Errorf("State = %v, want %v", decoded, state)
		}
	}
}

func TestRegistrationStatusType_AllValues(t *testing.T) {
	statuses := []RegistrationStatusType{
		RegistrationStatusAccepted,
		RegistrationStatusPending,
		RegistrationStatusRejected,
	}

	for _, status := range statuses {
		data, err := json.Marshal(status)
		if err != nil {
			t.Errorf("json.Marshal() error for %v = %v", status, err)
			continue
		}

		var decoded RegistrationStatusType
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			t.Errorf("json.Unmarshal() error for %v = %v", status, err)
			continue
		}

		if decoded != status {
			t.Errorf("Status = %v, want %v", decoded, status)
		}
	}
}

func TestStatusInfo_Serialization(t *testing.T) {
	info := StatusInfo{
		ReasonCode:     "Error123",
		AdditionalInfo: "Additional information here",
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded StatusInfo
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.ReasonCode != info.ReasonCode {
		t.Errorf("ReasonCode = %v, want %v", decoded.ReasonCode, info.ReasonCode)
	}
	if decoded.AdditionalInfo != info.AdditionalInfo {
		t.Errorf("AdditionalInfo = %v, want %v", decoded.AdditionalInfo, info.AdditionalInfo)
	}
}

func TestChargingProfile_Serialization(t *testing.T) {
	profile := ChargingProfile{
		Id:                     1,
		StackLevel:             0,
		ChargingProfilePurpose: ChargingProfilePurposeTxDefaultProfile,
		ChargingProfileKind:    ChargingProfileKindAbsolute,
		ChargingSchedule: []ChargingSchedule{
			{
				Id:               1,
				ChargingRateUnit: ChargingRateUnitW,
				ChargingSchedulePeriod: []ChargingSchedulePeriod{
					{
						StartPeriod: 0,
						Limit:       11000.0,
					},
				},
			},
		},
	}

	data, err := json.Marshal(profile)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded ChargingProfile
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.Id != profile.Id {
		t.Errorf("Id = %v, want %v", decoded.Id, profile.Id)
	}
	if len(decoded.ChargingSchedule) != 1 {
		t.Fatalf("ChargingSchedule length = %v, want 1", len(decoded.ChargingSchedule))
	}
	if decoded.ChargingSchedule[0].ChargingRateUnit != ChargingRateUnitW {
		t.Errorf("ChargingRateUnit = %v, want %v", decoded.ChargingSchedule[0].ChargingRateUnit, ChargingRateUnitW)
	}
}

// Helper function
func intPtr(i int) *int {
	return &i
}
