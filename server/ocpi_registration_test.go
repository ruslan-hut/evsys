package server

import (
	"evsys/internal"
	"testing"
)

type stubOcpi struct{}

func (stubOcpi) OnStatusNotification(*internal.EventMessage) {}
func (stubOcpi) OnTransactionStart(*internal.EventMessage)   {}
func (stubOcpi) OnTransactionStop(*internal.EventMessage)    {}
func (stubOcpi) OnAuthorize(*internal.EventMessage)          {}
func (stubOcpi) OnTransactionEvent(*internal.EventMessage)   {}
func (stubOcpi) OnAlert(*internal.EventMessage)              {}
func (stubOcpi) OnInfo(*internal.EventMessage)               {}
func (stubOcpi) Authorize(_, _, _ string) (bool, bool, bool, string) {
	return true, false, false, ""
}

func TestRegisterOcpi(t *testing.T) {
	tests := []struct {
		name          string
		disableEvents bool
		wantListeners int
	}{
		{"events on by default", false, 1},
		{"events disabled, authorization kept", true, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &SystemHandler{eventListeners: make([]internal.EventHandler, 0)}
			registerOcpi(h, stubOcpi{}, tt.disableEvents)
			if got := len(h.eventListeners); got != tt.wantListeners {
				t.Errorf("event listeners = %d, want %d", got, tt.wantListeners)
			}
			if h.auth == nil {
				t.Error("authorization service not set")
			}
		})
	}
}
