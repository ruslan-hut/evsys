package server

import (
	"reflect"
	"strings"
	"testing"

	"evsys/entity"
	"evsys/ocpp/v16/core"
	"evsys/types"
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

// The GetCompositeSchedule payload gained an object form so a diagnostic read
// can pin the unit; the bare-number form callers already use must keep working.
func TestParseCompositeScheduleQuery(t *testing.T) {
	tests := []struct {
		name     string
		payload  string
		duration int
		unit     types.ChargingRateUnitType
		wantErr  bool
	}{
		{name: "bare duration", payload: "3600", duration: 3600},
		{name: "bare duration with spaces", payload: " 3600 ", duration: 3600},
		{name: "object with unit", payload: `{"duration":600,"chargingRateUnit":"A"}`, duration: 600, unit: types.ChargingRateUnitAmperes},
		{name: "object with watts", payload: `{"duration":600,"chargingRateUnit":"W"}`, duration: 600, unit: types.ChargingRateUnitWatts},
		{name: "object without unit", payload: `{"duration":600}`, duration: 600},
		{name: "unsupported unit", payload: `{"duration":600,"chargingRateUnit":"kW"}`, wantErr: true},
		{name: "missing duration", payload: `{"chargingRateUnit":"A"}`, wantErr: true},
		{name: "negative duration", payload: `{"duration":-1}`, wantErr: true},
		{name: "not a payload at all", payload: "soon", wantErr: true},
		{name: "empty", payload: "", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			query, err := parseCompositeScheduleQuery(test.payload)
			if test.wantErr {
				if err == nil {
					t.Fatalf("parsed %q as %+v, want an error", test.payload, query)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseCompositeScheduleQuery(%q): %v", test.payload, err)
			}
			if query.Duration != test.duration {
				t.Errorf("duration = %d, want %d", query.Duration, test.duration)
			}
			if query.ChargingRateUnit != test.unit {
				t.Errorf("unit = %q, want %q", query.ChargingRateUnit, test.unit)
			}
		})
	}
}

// The GetConfiguration payload gained a comma-separated form so a diagnostic
// panel can ask for a handful of keys in one round trip instead of one call per
// key. The single-key and empty forms callers already use must keep working.
func TestOnGetConfigurationKeys(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    []string
	}{
		{name: "empty asks for everything", payload: "", want: []string{}},
		{name: "single key", payload: "ChargeProfileMaxStackLevel", want: []string{"ChargeProfileMaxStackLevel"}},
		{name: "several keys", payload: "ChargeProfileMaxStackLevel,MaxChargingProfilesInstalled",
			want: []string{"ChargeProfileMaxStackLevel", "MaxChargingProfilesInstalled"}},
		{name: "spaces around separators", payload: " A , B ", want: []string{"A", "B"}},
		{name: "empty entries dropped", payload: "A,,B,", want: []string{"A", "B"}},
		{name: "only separators", payload: ",,", want: []string{}},
	}

	h := &SystemHandler{
		chargePoints: map[string]*ChargePointState{"CP1": newChargePointState(&entity.ChargePoint{Id: "CP1"})},
		logger:       validationStubLogger{},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request, err := h.OnGetConfiguration("CP1", test.payload)
			if err != nil {
				t.Fatalf("OnGetConfiguration(%q): %v", test.payload, err)
			}
			if !reflect.DeepEqual(request.Key, test.want) {
				t.Errorf("keys = %#v, want %#v", request.Key, test.want)
			}
		})
	}
}

type validationStubLogger struct{}

func (validationStubLogger) FeatureEvent(_, _, _ string) {}
func (validationStubLogger) RawDataEvent(_, _ string)    {}
func (validationStubLogger) Debug(_ string)              {}
func (validationStubLogger) Warn(_ string)               {}
func (validationStubLogger) Error(_ string, _ error)     {}
