package availability

import (
	"encoding/json"
	"evsys/ocpp/v201"
	"testing"
	"time"
)

// ============================================================================
// OCPP 2.0.1 Availability Messages Tests
// ============================================================================
// Tests for StatusNotification
// ============================================================================

func TestStatusNotificationRequest_Serialization(t *testing.T) {
	now := time.Now()
	req := StatusNotificationRequest{
		Timestamp:       now,
		ConnectorStatus: v201.ConnectorStatusAvailable,
		EvseId:          1,
		ConnectorId:     1,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded StatusNotificationRequest
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.ConnectorStatus != req.ConnectorStatus {
		t.Errorf("ConnectorStatus = %v, want %v", decoded.ConnectorStatus, req.ConnectorStatus)
	}
	if decoded.EvseId != req.EvseId {
		t.Errorf("EvseId = %v, want %v", decoded.EvseId, req.EvseId)
	}
	if decoded.ConnectorId != req.ConnectorId {
		t.Errorf("ConnectorId = %v, want %v", decoded.ConnectorId, req.ConnectorId)
	}
	// Time comparison with tolerance
	if decoded.Timestamp.Sub(req.Timestamp).Abs() > time.Second {
		t.Errorf("Timestamp = %v, want %v", decoded.Timestamp, req.Timestamp)
	}
}

func TestStatusNotificationRequest_GetFeatureName(t *testing.T) {
	req := StatusNotificationRequest{}
	if req.GetFeatureName() != StatusNotificationFeatureName {
		t.Errorf("GetFeatureName() = %v, want %v", req.GetFeatureName(), StatusNotificationFeatureName)
	}
}

func TestStatusNotificationRequest_AllStatuses(t *testing.T) {
	statuses := []v201.ConnectorStatusType{
		v201.ConnectorStatusAvailable,
		v201.ConnectorStatusOccupied,
		v201.ConnectorStatusReserved,
		v201.ConnectorStatusUnavailable,
		v201.ConnectorStatusFaulted,
	}

	now := time.Now()
	for _, status := range statuses {
		req := StatusNotificationRequest{
			Timestamp:       now,
			ConnectorStatus: status,
			EvseId:          1,
			ConnectorId:     1,
		}

		data, err := json.Marshal(req)
		if err != nil {
			t.Errorf("json.Marshal() error for status %v = %v", status, err)
			continue
		}

		var decoded StatusNotificationRequest
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			t.Errorf("json.Unmarshal() error for status %v = %v", status, err)
			continue
		}

		if decoded.ConnectorStatus != status {
			t.Errorf("ConnectorStatus = %v, want %v", decoded.ConnectorStatus, status)
		}
	}
}

func TestStatusNotificationRequest_EVSE0(t *testing.T) {
	now := time.Now()
	req := StatusNotificationRequest{
		Timestamp:       now,
		ConnectorStatus: v201.ConnectorStatusAvailable,
		EvseId:          0, // Charging station itself
		ConnectorId:     0,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded StatusNotificationRequest
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.EvseId != 0 {
		t.Errorf("EvseId = %v, want 0", decoded.EvseId)
	}
	if decoded.ConnectorId != 0 {
		t.Errorf("ConnectorId = %v, want 0", decoded.ConnectorId)
	}
}

func TestStatusNotificationResponse_Serialization(t *testing.T) {
	resp := StatusNotificationResponse{}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded StatusNotificationResponse
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
}

func TestStatusNotificationResponse_GetFeatureName(t *testing.T) {
	resp := StatusNotificationResponse{}
	if resp.GetFeatureName() != StatusNotificationFeatureName {
		t.Errorf("GetFeatureName() = %v, want %v", resp.GetFeatureName(), StatusNotificationFeatureName)
	}
}

func TestStatusNotificationRequest_MultipleConnectors(t *testing.T) {
	now := time.Now()

	// Test EVSE 1, Connector 1
	req1 := StatusNotificationRequest{
		Timestamp:       now,
		ConnectorStatus: v201.ConnectorStatusAvailable,
		EvseId:          1,
		ConnectorId:     1,
	}

	// Test EVSE 1, Connector 2
	req2 := StatusNotificationRequest{
		Timestamp:       now,
		ConnectorStatus: v201.ConnectorStatusOccupied,
		EvseId:          1,
		ConnectorId:     2,
	}

	// Test EVSE 2, Connector 1
	req3 := StatusNotificationRequest{
		Timestamp:       now,
		ConnectorStatus: v201.ConnectorStatusReserved,
		EvseId:          2,
		ConnectorId:     1,
	}

	requests := []StatusNotificationRequest{req1, req2, req3}

	for i, req := range requests {
		data, err := json.Marshal(req)
		if err != nil {
			t.Errorf("Request %d: json.Marshal() error = %v", i, err)
			continue
		}

		var decoded StatusNotificationRequest
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			t.Errorf("Request %d: json.Unmarshal() error = %v", i, err)
			continue
		}

		if decoded.EvseId != req.EvseId {
			t.Errorf("Request %d: EvseId = %v, want %v", i, decoded.EvseId, req.EvseId)
		}
		if decoded.ConnectorId != req.ConnectorId {
			t.Errorf("Request %d: ConnectorId = %v, want %v", i, decoded.ConnectorId, req.ConnectorId)
		}
		if decoded.ConnectorStatus != req.ConnectorStatus {
			t.Errorf("Request %d: ConnectorStatus = %v, want %v", i, decoded.ConnectorStatus, req.ConnectorStatus)
		}
	}
}
