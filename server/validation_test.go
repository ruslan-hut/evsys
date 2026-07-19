package server

import (
	"reflect"
	"strings"
	"testing"

	"evsys/ocpp/v16/core"
)

// TestParseRawJsonRequestValidation verifies that schema validation runs on
// incoming OCPP payloads and rejects malformed messages while tolerating
// spec-optional fields such as the StatusNotification timestamp.
func TestParseRawJsonRequestValidation(t *testing.T) {
	tests := []struct {
		name      string
		reqType   reflect.Type
		payload   map[string]interface{}
		wantError bool
	}{
		{
			name:    "status notification without timestamp is accepted",
			reqType: reflect.TypeOf(core.StatusNotificationRequest{}),
			payload: map[string]interface{}{
				"connectorId": 1,
				"errorCode":   "NoError",
				"status":      "Available",
			},
			wantError: false,
		},
		{
			name:    "status notification with invalid status is rejected",
			reqType: reflect.TypeOf(core.StatusNotificationRequest{}),
			payload: map[string]interface{}{
				"connectorId": 1,
				"errorCode":   "NoError",
				"status":      "Bogus",
			},
			wantError: true,
		},
		{
			name:    "status notification with missing error code is rejected",
			reqType: reflect.TypeOf(core.StatusNotificationRequest{}),
			payload: map[string]interface{}{
				"connectorId": 1,
				"status":      "Available",
			},
			wantError: true,
		},
		{
			name:    "start transaction with connector 0 is rejected",
			reqType: reflect.TypeOf(core.StartTransactionRequest{}),
			payload: map[string]interface{}{
				"connectorId": 0,
				"idTag":       "ABC123",
				"meterStart":  0,
			},
			wantError: true,
		},
		{
			name:    "start transaction without idTag is rejected",
			reqType: reflect.TypeOf(core.StartTransactionRequest{}),
			payload: map[string]interface{}{
				"connectorId": 1,
				"meterStart":  0,
			},
			wantError: true,
		},
		{
			name:    "valid start transaction is accepted",
			reqType: reflect.TypeOf(core.StartTransactionRequest{}),
			payload: map[string]interface{}{
				"connectorId": 1,
				"idTag":       "ABC123",
				"meterStart":  0,
			},
			wantError: false,
		},
		{
			name:      "authorize without idTag is rejected",
			reqType:   reflect.TypeOf(core.AuthorizeRequest{}),
			payload:   map[string]interface{}{},
			wantError: true,
		},
		{
			name:    "meter values without samples is rejected",
			reqType: reflect.TypeOf(core.MeterValuesRequest{}),
			payload: map[string]interface{}{
				"connectorId": 1,
				"meterValue": []interface{}{
					map[string]interface{}{"sampledValue": []interface{}{}},
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseRawJsonRequest(tt.payload, tt.reqType)
			if tt.wantError && err == nil {
				t.Fatalf("expected validation error, got nil")
			}
			if !tt.wantError && err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if tt.wantError && err != nil && !strings.Contains(err.Error(), "validation failed") {
				t.Fatalf("expected validation error, got different error: %v", err)
			}
		})
	}
}
