package provisioning

import (
	"encoding/json"
	"evsys/ocpp/v201"
	"testing"
	"time"
)

// ============================================================================
// OCPP 2.0.1 Provisioning Messages Tests
// ============================================================================
// Tests for BootNotification, Heartbeat, NotifyReport, etc.
// ============================================================================

func TestBootNotificationRequest_Serialization(t *testing.T) {
	req := BootNotificationRequest{
		Reason: v201.BootReasonPowerUp,
		ChargingStation: v201.ChargingStation{
			Model:           "Model X",
			VendorName:      "Vendor Inc",
			SerialNumber:    "SN123456",
			FirmwareVersion: "1.0.0",
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded BootNotificationRequest
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.Reason != req.Reason {
		t.Errorf("Reason = %v, want %v", decoded.Reason, req.Reason)
	}
	if decoded.ChargingStation.Model != req.ChargingStation.Model {
		t.Errorf("ChargingStation.Model = %v, want %v", decoded.ChargingStation.Model, req.ChargingStation.Model)
	}
}

func TestBootNotificationRequest_GetFeatureName(t *testing.T) {
	req := BootNotificationRequest{}
	if req.GetFeatureName() != BootNotificationFeatureName {
		t.Errorf("GetFeatureName() = %v, want %v", req.GetFeatureName(), BootNotificationFeatureName)
	}
}

func TestBootNotificationResponse_Serialization(t *testing.T) {
	now := time.Now()
	resp := BootNotificationResponse{
		Status:      v201.RegistrationStatusAccepted,
		CurrentTime: now,
		Interval:    300,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded BootNotificationResponse
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.Status != resp.Status {
		t.Errorf("Status = %v, want %v", decoded.Status, resp.Status)
	}
	if decoded.Interval != resp.Interval {
		t.Errorf("Interval = %v, want %v", decoded.Interval, resp.Interval)
	}
	// Time comparison with tolerance
	if decoded.CurrentTime.Sub(resp.CurrentTime).Abs() > time.Second {
		t.Errorf("CurrentTime = %v, want %v", decoded.CurrentTime, resp.CurrentTime)
	}
}

func TestBootNotificationResponse_GetFeatureName(t *testing.T) {
	resp := BootNotificationResponse{}
	if resp.GetFeatureName() != BootNotificationFeatureName {
		t.Errorf("GetFeatureName() = %v, want %v", resp.GetFeatureName(), BootNotificationFeatureName)
	}
}

func TestBootReasonType_AllValues(t *testing.T) {
	reasons := []v201.BootReasonType{
		v201.BootReasonApplicationReset,
		v201.BootReasonFirmwareUpdate,
		v201.BootReasonLocalReset,
		v201.BootReasonPowerUp,
		v201.BootReasonRemoteReset,
		v201.BootReasonScheduledReset,
		v201.BootReasonTriggered,
		v201.BootReasonUnknown,
		v201.BootReasonWatchdog,
	}

	for _, reason := range reasons {
		data, err := json.Marshal(reason)
		if err != nil {
			t.Errorf("json.Marshal() error for %v = %v", reason, err)
			continue
		}

		var decoded v201.BootReasonType
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

func TestHeartbeatRequest_Serialization(t *testing.T) {
	req := HeartbeatRequest{}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded HeartbeatRequest
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
}

func TestHeartbeatRequest_GetFeatureName(t *testing.T) {
	req := HeartbeatRequest{}
	if req.GetFeatureName() != HeartbeatFeatureName {
		t.Errorf("GetFeatureName() = %v, want %v", req.GetFeatureName(), HeartbeatFeatureName)
	}
}

func TestHeartbeatResponse_Serialization(t *testing.T) {
	now := time.Now()
	resp := HeartbeatResponse{
		CurrentTime: now,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded HeartbeatResponse
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.CurrentTime.Sub(resp.CurrentTime).Abs() > time.Second {
		t.Errorf("CurrentTime = %v, want %v", decoded.CurrentTime, resp.CurrentTime)
	}
}

func TestNotifyReportRequest_Serialization(t *testing.T) {
	now := time.Now()
	req := NotifyReportRequest{
		RequestId:   123,
		GeneratedAt: now,
		SeqNo:       1,
		Tbc:         false,
		ReportData:  []ReportData{},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded NotifyReportRequest
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.RequestId != req.RequestId {
		t.Errorf("RequestId = %v, want %v", decoded.RequestId, req.RequestId)
	}
	if decoded.SeqNo != req.SeqNo {
		t.Errorf("SeqNo = %v, want %v", decoded.SeqNo, req.SeqNo)
	}
	if decoded.Tbc != req.Tbc {
		t.Errorf("Tbc = %v, want %v", decoded.Tbc, req.Tbc)
	}
}

func TestNotifyReportRequest_GetFeatureName(t *testing.T) {
	req := NotifyReportRequest{}
	if req.GetFeatureName() != NotifyReportFeatureName {
		t.Errorf("GetFeatureName() = %v, want %v", req.GetFeatureName(), NotifyReportFeatureName)
	}
}

func TestNotifyReportResponse_Serialization(t *testing.T) {
	resp := NotifyReportResponse{}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded NotifyReportResponse
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
}

func TestGetBaseReportRequest_Serialization(t *testing.T) {
	req := GetBaseReportRequest{
		RequestId:  456,
		ReportBase: ReportBaseConfigurationInventory,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded GetBaseReportRequest
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.RequestId != req.RequestId {
		t.Errorf("RequestId = %v, want %v", decoded.RequestId, req.RequestId)
	}
	if decoded.ReportBase != req.ReportBase {
		t.Errorf("ReportBase = %v, want %v", decoded.ReportBase, req.ReportBase)
	}
}

func TestGetBaseReportRequest_GetFeatureName(t *testing.T) {
	req := GetBaseReportRequest{}
	if req.GetFeatureName() != GetBaseReportFeatureName {
		t.Errorf("GetFeatureName() = %v, want %v", req.GetFeatureName(), GetBaseReportFeatureName)
	}
}

func TestResetRequest_Serialization(t *testing.T) {
	evseId := 1
	req := ResetRequest{
		Type:   ResetTypeImmediate,
		EvseId: &evseId,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded ResetRequest
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.Type != req.Type {
		t.Errorf("Type = %v, want %v", decoded.Type, req.Type)
	}
	if decoded.EvseId == nil || *decoded.EvseId != *req.EvseId {
		t.Errorf("EvseId = %v, want %v", decoded.EvseId, req.EvseId)
	}
}

func TestResetRequest_GetFeatureName(t *testing.T) {
	req := ResetRequest{}
	if req.GetFeatureName() != ResetFeatureName {
		t.Errorf("GetFeatureName() = %v, want %v", req.GetFeatureName(), ResetFeatureName)
	}
}

func TestResetType_AllValues(t *testing.T) {
	types := []ResetType{
		ResetTypeImmediate,
		ResetTypeOnIdle,
	}

	for _, resetType := range types {
		data, err := json.Marshal(resetType)
		if err != nil {
			t.Errorf("json.Marshal() error for %v = %v", resetType, err)
			continue
		}

		var decoded ResetType
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			t.Errorf("json.Unmarshal() error for %v = %v", resetType, err)
			continue
		}

		if decoded != resetType {
			t.Errorf("Type = %v, want %v", decoded, resetType)
		}
	}
}

func TestReportBaseType_AllValues(t *testing.T) {
	types := []ReportBaseType{
		ReportBaseConfigurationInventory,
		ReportBaseFullInventory,
		ReportBaseSummaryInventory,
	}

	for _, reportType := range types {
		data, err := json.Marshal(reportType)
		if err != nil {
			t.Errorf("json.Marshal() error for %v = %v", reportType, err)
			continue
		}

		var decoded ReportBaseType
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			t.Errorf("json.Unmarshal() error for %v = %v", reportType, err)
			continue
		}

		if decoded != reportType {
			t.Errorf("Type = %v, want %v", decoded, reportType)
		}
	}
}
