package transactions

import (
	"encoding/json"
	"evsys/ocpp/v201"
	"testing"
	"time"
)

// ============================================================================
// OCPP 2.0.1 Transaction Messages Tests
// ============================================================================
// Tests for TransactionEvent
// ============================================================================

func TestTransactionEventRequest_Serialization_Started(t *testing.T) {
	now := time.Now()
	connectorId := 1
	req := TransactionEventRequest{
		EventType:     v201.TransactionEventStarted,
		Timestamp:     now,
		TriggerReason: TriggerReasonCablePluggedIn,
		SeqNo:         0,
		TransactionInfo: v201.Transaction{
			TransactionId: "TX12345",
			ChargingState: v201.ChargingStateEVConnected,
		},
		IdToken: &v201.IdToken{
			IdToken: "TESTTOKEN",
			Type:    v201.IdTokenTypeISO14443,
		},
		Evse: &v201.EVSE{
			Id:          1,
			ConnectorId: &connectorId,
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded TransactionEventRequest
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.EventType != req.EventType {
		t.Errorf("EventType = %v, want %v", decoded.EventType, req.EventType)
	}
	if decoded.TriggerReason != req.TriggerReason {
		t.Errorf("TriggerReason = %v, want %v", decoded.TriggerReason, req.TriggerReason)
	}
	if decoded.TransactionInfo.TransactionId != req.TransactionInfo.TransactionId {
		t.Errorf("TransactionId = %v, want %v", decoded.TransactionInfo.TransactionId, req.TransactionInfo.TransactionId)
	}
	if decoded.IdToken == nil || decoded.IdToken.IdToken != req.IdToken.IdToken {
		t.Errorf("IdToken = %v, want %v", decoded.IdToken, req.IdToken)
	}
}

func TestTransactionEventRequest_Serialization_Updated(t *testing.T) {
	now := time.Now()
	req := TransactionEventRequest{
		EventType:     v201.TransactionEventUpdated,
		Timestamp:     now,
		TriggerReason: TriggerReasonMeterValuePeriodic,
		SeqNo:         5,
		TransactionInfo: v201.Transaction{
			TransactionId: "TX12345",
			ChargingState: v201.ChargingStateCharging,
		},
		MeterValue: []v201.MeterValue{
			{
				Timestamp: now,
				SampledValue: []v201.SampledValue{
					{
						Value:     12345.0,
						Measurand: v201.MeasurandEnergyActiveImportRegister,
						Context:   v201.ReadingContextSamplePeriodic,
					},
				},
			},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded TransactionEventRequest
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.EventType != req.EventType {
		t.Errorf("EventType = %v, want %v", decoded.EventType, req.EventType)
	}
	if decoded.SeqNo != req.SeqNo {
		t.Errorf("SeqNo = %v, want %v", decoded.SeqNo, req.SeqNo)
	}
	if len(decoded.MeterValue) != 1 {
		t.Fatalf("MeterValue length = %v, want 1", len(decoded.MeterValue))
	}
	if decoded.MeterValue[0].SampledValue[0].Value != 12345.0 {
		t.Errorf("MeterValue.Value = %v, want 12345.0", decoded.MeterValue[0].SampledValue[0].Value)
	}
}

func TestTransactionEventRequest_Serialization_Ended(t *testing.T) {
	now := time.Now()
	req := TransactionEventRequest{
		EventType:     v201.TransactionEventEnded,
		Timestamp:     now,
		TriggerReason: TriggerReasonEVCommunicationLost,
		SeqNo:         10,
		TransactionInfo: v201.Transaction{
			TransactionId: "TX12345",
			ChargingState: v201.ChargingStateSuspendedEV,
			StoppedReason: StoppedReasonLocal,
		},
		MeterValue: []v201.MeterValue{
			{
				Timestamp: now,
				SampledValue: []v201.SampledValue{
					{
						Value:     25000.0,
						Measurand: v201.MeasurandEnergyActiveImportRegister,
						Context:   v201.ReadingContextTransactionEnd,
					},
				},
			},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded TransactionEventRequest
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.EventType != req.EventType {
		t.Errorf("EventType = %v, want %v", decoded.EventType, req.EventType)
	}
	if decoded.TransactionInfo.StoppedReason != req.TransactionInfo.StoppedReason {
		t.Errorf("StoppedReason = %v, want %v", decoded.TransactionInfo.StoppedReason, req.TransactionInfo.StoppedReason)
	}
}

func TestTransactionEventRequest_GetFeatureName(t *testing.T) {
	req := TransactionEventRequest{}
	if req.GetFeatureName() != TransactionEventFeatureName {
		t.Errorf("GetFeatureName() = %v, want %v", req.GetFeatureName(), TransactionEventFeatureName)
	}
}

func TestTransactionEventResponse_Serialization(t *testing.T) {
	resp := TransactionEventResponse{
		TotalCost:        floatPtr(25.50),
		ChargingPriority: intPtr(1),
		IdTokenInfo: &v201.IdTokenInfo{
			Status: v201.AuthorizationStatusAccepted,
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded TransactionEventResponse
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.TotalCost == nil || *decoded.TotalCost != *resp.TotalCost {
		t.Errorf("TotalCost = %v, want %v", decoded.TotalCost, resp.TotalCost)
	}
	if decoded.IdTokenInfo == nil || decoded.IdTokenInfo.Status != v201.AuthorizationStatusAccepted {
		t.Errorf("IdTokenInfo.Status = %v, want Accepted", decoded.IdTokenInfo)
	}
}

func TestTransactionEventResponse_GetFeatureName(t *testing.T) {
	resp := TransactionEventResponse{}
	if resp.GetFeatureName() != TransactionEventFeatureName {
		t.Errorf("GetFeatureName() = %v, want %v", resp.GetFeatureName(), TransactionEventFeatureName)
	}
}

func TestTriggerReasonType_AllValues(t *testing.T) {
	reasons := []TriggerReasonType{
		TriggerReasonAuthorized,
		TriggerReasonCablePluggedIn,
		TriggerReasonChargingRateChanged,
		TriggerReasonChargingStateChanged,
		TriggerReasonDeauthorized,
		TriggerReasonEnergyLimitReached,
		TriggerReasonEVCommunicationLost,
		TriggerReasonEVConnectTimeout,
		TriggerReasonMeterValueClock,
		TriggerReasonMeterValuePeriodic,
		TriggerReasonTimeLimitReached,
		TriggerReasonTrigger,
		TriggerReasonUnlockCommand,
		TriggerReasonStopAuthorized,
		TriggerReasonEVDeparted,
		TriggerReasonEVDetected,
		TriggerReasonRemoteStop,
		TriggerReasonRemoteStart,
		TriggerReasonAbnormalCondition,
		TriggerReasonSignedDataReceived,
		TriggerReasonResetCommand,
	}

	for _, reason := range reasons {
		data, err := json.Marshal(reason)
		if err != nil {
			t.Errorf("json.Marshal() error for %v = %v", reason, err)
			continue
		}

		var decoded TriggerReasonType
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			t.Errorf("json.Unmarshal() error for %v = %v", reason, err)
			continue
		}

		if decoded != reason {
			t.Errorf("Reason = %v, want %v", decoded, reason)
		}
	}
}

func TestStoppedReasonType_AllValues(t *testing.T) {
	reasons := []StoppedReasonType{
		StoppedReasonDeAuthorized,
		StoppedReasonEmergencyStop,
		StoppedReasonEVDisconnected,
		StoppedReasonGroundFault,
		StoppedReasonImmediateReset,
		StoppedReasonLocal,
		StoppedReasonLocalOutOfCredit,
		StoppedReasonMasterPass,
		StoppedReasonOther,
		StoppedReasonOvercurrentFault,
		StoppedReasonPowerLoss,
		StoppedReasonPowerQuality,
		StoppedReasonReboot,
		StoppedReasonRemote,
		StoppedReasonSOCLimitReached,
		StoppedReasonStoppedByEV,
		StoppedReasonTimeLimitReached,
		StoppedReasonTimeout,
	}

	for _, reason := range reasons {
		data, err := json.Marshal(reason)
		if err != nil {
			t.Errorf("json.Marshal() error for %v = %v", reason, err)
			continue
		}

		var decoded StoppedReasonType
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			t.Errorf("json.Unmarshal() error for %v = %v", reason, err)
			continue
		}

		if decoded != reason {
			t.Errorf("Reason = %v, want %v", decoded, reason)
		}
	}
}

// Helper functions
func floatPtr(f float64) *float64 {
	return &f
}

func intPtr(i int) *int {
	return &i
}
