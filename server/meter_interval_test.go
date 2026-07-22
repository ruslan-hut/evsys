package server

import (
	"fmt"
	"testing"

	"evsys/ocpp"
	"evsys/ocpp/v16/core"
)

// sentRequest records what enforceMeterValueInterval pushed to a charge point.
type recordingSender struct {
	clientId string
	request  ocpp.Request
	calls    int
	err      error
}

func (s *recordingSender) SendRequest(clientId string, request ocpp.Request) (string, error) {
	s.calls++
	s.clientId = clientId
	s.request = request
	return "id", s.err
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
