package authorization

import (
	"encoding/json"
	"evsys/ocpp/v201"
	"testing"
)

// ============================================================================
// OCPP 2.0.1 Authorization Messages Tests
// ============================================================================
// Tests for Authorize, ClearedChargingLimit
// ============================================================================

func TestAuthorizeRequest_Serialization(t *testing.T) {
	req := AuthorizeRequest{
		IdToken: v201.IdToken{
			IdToken: "TESTTOKEN123",
			Type:    v201.IdTokenTypeISO14443,
		},
		Certificate: "CERTIFICATE_DATA_HERE",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded AuthorizeRequest
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.IdToken.IdToken != req.IdToken.IdToken {
		t.Errorf("IdToken.IdToken = %v, want %v", decoded.IdToken.IdToken, req.IdToken.IdToken)
	}
	if decoded.IdToken.Type != req.IdToken.Type {
		t.Errorf("IdToken.Type = %v, want %v", decoded.IdToken.Type, req.IdToken.Type)
	}
	if decoded.Certificate != req.Certificate {
		t.Errorf("Certificate = %v, want %v", decoded.Certificate, req.Certificate)
	}
}

func TestAuthorizeRequest_GetFeatureName(t *testing.T) {
	req := AuthorizeRequest{}
	if req.GetFeatureName() != AuthorizeFeatureName {
		t.Errorf("GetFeatureName() = %v, want %v", req.GetFeatureName(), AuthorizeFeatureName)
	}
}

func TestAuthorizeResponse_Serialization(t *testing.T) {
	resp := AuthorizeResponse{
		IdTokenInfo: v201.IdTokenInfo{
			Status: v201.AuthorizationStatusAccepted,
		},
		CertificateStatus: v201.AuthorizeCertificateStatusAccepted,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded AuthorizeResponse
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.IdTokenInfo.Status != resp.IdTokenInfo.Status {
		t.Errorf("IdTokenInfo.Status = %v, want %v", decoded.IdTokenInfo.Status, resp.IdTokenInfo.Status)
	}
	if decoded.CertificateStatus != resp.CertificateStatus {
		t.Errorf("CertificateStatus = %v, want %v", decoded.CertificateStatus, resp.CertificateStatus)
	}
}

func TestAuthorizeResponse_GetFeatureName(t *testing.T) {
	resp := AuthorizeResponse{}
	if resp.GetFeatureName() != AuthorizeFeatureName {
		t.Errorf("GetFeatureName() = %v, want %v", resp.GetFeatureName(), AuthorizeFeatureName)
	}
}

func TestAuthorizeResponse_AllAuthorizationStatuses(t *testing.T) {
	statuses := []v201.AuthorizationStatusType{
		v201.AuthorizationStatusAccepted,
		v201.AuthorizationStatusBlocked,
		v201.AuthorizationStatusConcurrentTx,
		v201.AuthorizationStatusExpired,
		v201.AuthorizationStatusInvalid,
		v201.AuthorizationStatusNoCredit,
		v201.AuthorizationStatusNotAllowedTypeEVSE,
		v201.AuthorizationStatusNotAtThisLocation,
		v201.AuthorizationStatusNotAtThisTime,
		v201.AuthorizationStatusUnknown,
	}

	for _, status := range statuses {
		resp := AuthorizeResponse{
			IdTokenInfo: v201.IdTokenInfo{
				Status: status,
			},
		}

		data, err := json.Marshal(resp)
		if err != nil {
			t.Errorf("json.Marshal() error for status %v = %v", status, err)
			continue
		}

		var decoded AuthorizeResponse
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			t.Errorf("json.Unmarshal() error for status %v = %v", status, err)
			continue
		}

		if decoded.IdTokenInfo.Status != status {
			t.Errorf("Status = %v, want %v", decoded.IdTokenInfo.Status, status)
		}
	}
}

func TestClearedChargingLimitRequest_Serialization(t *testing.T) {
	evseId := 1
	req := ClearedChargingLimitRequest{
		ChargingLimitSource: ChargingLimitSourceEMS,
		EvseId:              &evseId,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded ClearedChargingLimitRequest
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.ChargingLimitSource != req.ChargingLimitSource {
		t.Errorf("ChargingLimitSource = %v, want %v", decoded.ChargingLimitSource, req.ChargingLimitSource)
	}
	if decoded.EvseId == nil || *decoded.EvseId != *req.EvseId {
		t.Errorf("EvseId = %v, want %v", decoded.EvseId, req.EvseId)
	}
}

func TestClearedChargingLimitRequest_GetFeatureName(t *testing.T) {
	req := ClearedChargingLimitRequest{}
	if req.GetFeatureName() != ClearedChargingLimitFeatureName {
		t.Errorf("GetFeatureName() = %v, want %v", req.GetFeatureName(), ClearedChargingLimitFeatureName)
	}
}

func TestClearedChargingLimitResponse_Serialization(t *testing.T) {
	resp := ClearedChargingLimitResponse{}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded ClearedChargingLimitResponse
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
}

func TestClearedChargingLimitResponse_GetFeatureName(t *testing.T) {
	resp := ClearedChargingLimitResponse{}
	if resp.GetFeatureName() != ClearedChargingLimitFeatureName {
		t.Errorf("GetFeatureName() = %v, want %v", resp.GetFeatureName(), ClearedChargingLimitFeatureName)
	}
}

func TestChargingLimitSourceType_AllValues(t *testing.T) {
	sources := []ChargingLimitSourceType{
		ChargingLimitSourceEMS,
		ChargingLimitSourceOther,
		ChargingLimitSourceSO,
		ChargingLimitSourceCSO,
	}

	for _, source := range sources {
		data, err := json.Marshal(source)
		if err != nil {
			t.Errorf("json.Marshal() error for %v = %v", source, err)
			continue
		}

		var decoded ChargingLimitSourceType
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			t.Errorf("json.Unmarshal() error for %v = %v", source, err)
			continue
		}

		if decoded != source {
			t.Errorf("Source = %v, want %v", decoded, source)
		}
	}
}

func TestCertificateStatusType_AllValues(t *testing.T) {
	statuses := []v201.AuthorizeCertificateStatusType{
		v201.AuthorizeCertificateStatusAccepted,
		v201.AuthorizeCertificateStatusSignatureError,
		v201.AuthorizeCertificateStatusCertificateExpired,
		v201.AuthorizeCertificateStatusCertificateRevoked,
		v201.AuthorizeCertificateStatusNoCertificateAvailable,
		v201.AuthorizeCertificateStatusCertChainError,
		v201.AuthorizeCertificateStatusContractCancelled,
	}

	for _, status := range statuses {
		data, err := json.Marshal(status)
		if err != nil {
			t.Errorf("json.Marshal() error for %v = %v", status, err)
			continue
		}

		var decoded v201.AuthorizeCertificateStatusType
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
