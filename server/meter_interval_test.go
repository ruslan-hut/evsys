package server

import (
	"fmt"
	"testing"
	"time"

	"evsys/ocpp"
	"evsys/ocpp/v16/core"
)

// sentRequest records what enforceMeterValueInterval pushed to a charge point.
type recordingSender struct {
	clientId string
	request  ocpp.Request
	calls    int
	err      error
	// syncResponse is what SendRequestSync returns; syncErr overrides it.
	syncResponse string
	syncErr      error
	syncRequests []ocpp.Request
}

func (s *recordingSender) SendRequest(clientId string, request ocpp.Request) (string, error) {
	s.calls++
	s.clientId = clientId
	s.request = request
	return "id", s.err
}

func (s *recordingSender) SendRequestSync(_ string, request ocpp.Request, _ time.Duration) (string, error) {
	s.syncRequests = append(s.syncRequests, request)
	return s.syncResponse, s.syncErr
}

func TestEnforceMeterValueInterval(t *testing.T) {
	t.Run("pushes the configured interval when triggering is off", func(t *testing.T) {
		sender := &recordingSender{}
		h := &SystemHandler{
			server:              sender,
			meterSampleInterval: 60,
			logger:              meterStubLogger{},
		}

		h.enforceMeterValueInterval("CP1", false)

		if sender.calls != 1 {
			t.Fatalf("SendRequest called %d times, want 1", sender.calls)
		}
		if sender.clientId != "CP1" {
			t.Errorf("clientId = %q, want CP1", sender.clientId)
		}
		req, ok := sender.request.(*core.ChangeConfigurationRequest)
		if !ok {
			t.Fatalf("request type = %T, want *core.ChangeConfigurationRequest", sender.request)
		}
		if req.Key != "MeterValueSampleInterval" {
			t.Errorf("key = %q, want MeterValueSampleInterval", req.Key)
		}
		if req.Value != "60" {
			t.Errorf("value = %q, want 60", req.Value)
		}
	})

	t.Run("skips charge points with triggering on", func(t *testing.T) {
		sender := &recordingSender{}
		h := &SystemHandler{
			server:              sender,
			meterSampleInterval: 60,
			logger:              meterStubLogger{},
		}

		h.enforceMeterValueInterval("CP1", true)

		if sender.calls != 0 {
			t.Fatalf("SendRequest called %d times, want 0", sender.calls)
		}
	})

	t.Run("does nothing when interval is not configured", func(t *testing.T) {
		sender := &recordingSender{}
		h := &SystemHandler{
			server:              sender,
			meterSampleInterval: 0,
			logger:              meterStubLogger{},
		}

		h.enforceMeterValueInterval("CP1", false)

		if sender.calls != 0 {
			t.Fatalf("SendRequest called %d times, want 0", sender.calls)
		}
	})

	t.Run("does not panic when no sender is wired", func(t *testing.T) {
		h := &SystemHandler{
			meterSampleInterval: 60,
			logger:              meterStubLogger{},
		}
		h.enforceMeterValueInterval("CP1", false) // server nil, must be a no-op
	})

	t.Run("logs the failure but does not propagate it", func(t *testing.T) {
		sender := &recordingSender{err: fmt.Errorf("not available")}
		h := &SystemHandler{
			server:              sender,
			meterSampleInterval: 30,
			logger:              meterStubLogger{},
		}
		h.enforceMeterValueInterval("CP1", false) // must not panic on the send error
		if sender.calls != 1 {
			t.Fatalf("SendRequest called %d times, want 1", sender.calls)
		}
	})
}

func TestMergeMeasurands(t *testing.T) {
	tests := []struct {
		name        string
		current     string
		required    []string
		wantValue   string
		wantChanged bool
	}{
		{
			name:        "empty list gets all required",
			current:     "",
			required:    []string{"Voltage", "Current.Import", "Current.Offered"},
			wantValue:   "Voltage,Current.Import,Current.Offered",
			wantChanged: true,
		},
		{
			name:        "keeps the charge point's own entries and appends",
			current:     "Energy.Active.Import.Register,SoC",
			required:    []string{"Voltage", "Current.Offered"},
			wantValue:   "Energy.Active.Import.Register,SoC,Voltage,Current.Offered",
			wantChanged: true,
		},
		{
			name:        "no change when everything is already present",
			current:     "Energy.Active.Import.Register,Voltage,Current.Offered",
			required:    []string{"Voltage", "Current.Offered"},
			wantValue:   "Energy.Active.Import.Register,Voltage,Current.Offered",
			wantChanged: false,
		},
		{
			name:        "measurand names compare case-insensitively",
			current:     "voltage",
			required:    []string{"Voltage"},
			wantValue:   "voltage",
			wantChanged: false,
		},
		{
			name:        "tolerates spaces around entries",
			current:     "Energy.Active.Import.Register , SoC ",
			required:    []string{"Voltage"},
			wantValue:   "Energy.Active.Import.Register,SoC,Voltage",
			wantChanged: true,
		},
		{
			name:        "only the missing ones are added",
			current:     "Voltage",
			required:    []string{"Voltage", "Current.Import"},
			wantValue:   "Voltage,Current.Import",
			wantChanged: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, changed := mergeMeasurands(test.current, test.required)
			if value != test.wantValue {
				t.Errorf("value = %q, want %q", value, test.wantValue)
			}
			if changed != test.wantChanged {
				t.Errorf("changed = %v, want %v", changed, test.wantChanged)
			}
		})
	}
}

func configResponseFor(key, value string) string {
	return fmt.Sprintf(`{"configurationKey":[{"key":%q,"readonly":false,"value":%q}]}`, key, value)
}

func TestEnforceMeterMeasurands(t *testing.T) {
	required := []string{"Voltage", "Current.Import", "Current.Offered"}

	t.Run("adds the missing measurands to what the charge point reports", func(t *testing.T) {
		sender := &recordingSender{
			syncResponse: configResponseFor("MeterValuesSampledData", "Energy.Active.Import.Register"),
		}
		h := &SystemHandler{server: sender, meterMeasurands: required, logger: meterStubLogger{}}

		h.enforceMeterMeasurands("CP1")

		if len(sender.syncRequests) != 1 {
			t.Fatalf("read the config %d times, want 1", len(sender.syncRequests))
		}
		if sender.calls != 1 {
			t.Fatalf("ChangeConfiguration sent %d times, want 1", sender.calls)
		}
		req, ok := sender.request.(*core.ChangeConfigurationRequest)
		if !ok {
			t.Fatalf("request type = %T", sender.request)
		}
		if req.Key != "MeterValuesSampledData" {
			t.Errorf("key = %q, want MeterValuesSampledData", req.Key)
		}
		want := "Energy.Active.Import.Register,Voltage,Current.Import,Current.Offered"
		if req.Value != want {
			t.Errorf("value = %q, want %q", req.Value, want)
		}
	})

	t.Run("does not write when the measurands are already reported", func(t *testing.T) {
		sender := &recordingSender{
			syncResponse: configResponseFor("MeterValuesSampledData",
				"Energy.Active.Import.Register,Voltage,Current.Import,Current.Offered"),
		}
		h := &SystemHandler{server: sender, meterMeasurands: required, logger: meterStubLogger{}}

		h.enforceMeterMeasurands("CP1")

		if sender.calls != 0 {
			t.Fatalf("ChangeConfiguration sent %d times, want 0 (nothing missing)", sender.calls)
		}
	})

	t.Run("does nothing when no measurands are configured", func(t *testing.T) {
		sender := &recordingSender{}
		h := &SystemHandler{server: sender, meterMeasurands: nil, logger: meterStubLogger{}}

		h.enforceMeterMeasurands("CP1")

		if len(sender.syncRequests) != 0 || sender.calls != 0 {
			t.Fatalf("touched the charge point with no measurands configured: %d reads, %d writes",
				len(sender.syncRequests), sender.calls)
		}
	})

	t.Run("does not write when the config read fails", func(t *testing.T) {
		sender := &recordingSender{syncErr: fmt.Errorf("not available")}
		h := &SystemHandler{server: sender, meterMeasurands: required, logger: meterStubLogger{}}

		h.enforceMeterMeasurands("CP1")

		if sender.calls != 0 {
			t.Fatalf("wrote config after a failed read: %d writes", sender.calls)
		}
	})

	t.Run("does not panic when no sender is wired", func(t *testing.T) {
		h := &SystemHandler{meterMeasurands: required, logger: meterStubLogger{}}
		h.enforceMeterMeasurands("CP1") // server nil, must be a no-op
	})
}
