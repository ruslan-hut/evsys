package power

import (
	"evsys/entity"
	"evsys/ocpp"
	"strings"
	"sync"
	"testing"
	"time"
)

type stubRepo struct {
	chargePoint *entity.ChargePoint
	location    *entity.Location
	txLimits    map[int]int // transactionId -> last recorded power limit
}

func (s *stubRepo) GetChargePoint(_ string) (*entity.ChargePoint, error) {
	return s.chargePoint, nil
}

func (s *stubRepo) GetLocation(_ string) (*entity.Location, error) {
	return s.location, nil
}

func (s *stubRepo) GetLocations() ([]*entity.Location, error) {
	return []*entity.Location{s.location}, nil
}

func (s *stubRepo) UpdateConnectorCurrentPower(_ *entity.Connector) error {
	return nil
}

func (s *stubRepo) UpdateTransactionPowerLimit(transactionId, limit int) error {
	if s.txLimits == nil {
		s.txLimits = make(map[int]int)
	}
	s.txLimits[transactionId] = limit
	return nil
}

type stubServer struct {
	// payload is the CallResult the stub charge point answers with; empty means
	// it accepts the profile.
	payload string
}

func (s *stubServer) SendRequest(_ string, _ ocpp.Request) (string, error) {
	return "", nil
}

// SendRequestWithResponse answers immediately so the verdict goroutine the load
// balancer spawns finishes within the test rather than sitting on its timeout.
func (s *stubServer) SendRequestWithResponse(_ string, _ ocpp.Request) (<-chan string, func(), error) {
	payload := s.payload
	if payload == "" {
		payload = `{"status":"Accepted"}`
	}
	response := make(chan string, 1)
	response <- payload
	return response, func() {}, nil
}

// stubLog records feature events so a test can assert on what the load balancer
// reported. The verdict goroutine writes concurrently with the test, hence the
// mutex.
type stubLog struct {
	mutex  sync.Mutex
	events []string
}

func (s *stubLog) FeatureEvent(_, _, text string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.events = append(s.events, text)
}

func (s *stubLog) RawDataEvent(_, _ string) {}
func (s *stubLog) Debug(_ string)           {}
func (s *stubLog) Warn(_ string)            {}
func (s *stubLog) Error(_ string, _ error)  {}

// waitForEvent blocks until a recorded event contains substring, or fails the
// test. The event is produced on the goroutine that waits for the charge point's
// verdict, so it does not exist yet when CheckPowerLimit returns.
func (s *stubLog) waitForEvent(t *testing.T, substring string) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s.mutex.Lock()
		for _, event := range s.events {
			if strings.Contains(event, substring) {
				s.mutex.Unlock()
				return event
			}
		}
		s.mutex.Unlock()
		time.Sleep(time.Millisecond)
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	t.Fatalf("no event containing %q; recorded: %v", substring, s.events)
	return ""
}

func newTestBalancer(connectorCount int) (*LoadBalancer, []*entity.Connector) {
	connectors := make([]*entity.Connector, connectorCount)
	for i := range connectors {
		connectors[i] = entity.NewConnector(i+1, "chp1")
	}
	chp := &entity.ChargePoint{
		Id:            "chp1",
		LocationId:    "loc1",
		SmartCharging: true,
		Connectors:    connectors,
	}
	location := &entity.Location{
		Id:         "loc1",
		PowerLimit: 500,
		Evses:      []*entity.ChargePoint{chp},
	}
	repo := &stubRepo{chargePoint: chp, location: location}
	lb := NewLoadBalancer(repo, &stubServer{}, &stubLog{})
	return lb, connectors
}

// A charge point that rejects a charging profile keeps charging at whatever its
// own hardware allows, which looks exactly like a limit correctly applied. The
// verdict has to reach the log or the load balancer is unfalsifiable.
func TestRejectedProfileIsReported(t *testing.T) {
	lb, connectors := newTestBalancer(1)
	lb.server.(*stubServer).payload = `{"status":"Rejected"}`
	log := lb.log.(*stubLog)

	connectors[0].CurrentTransactionId = 7
	lb.CheckPowerLimit("chp1")

	event := log.waitForEvent(t, "REJECTED")
	if !strings.Contains(event, "Rejected") {
		t.Fatalf("event %q does not carry the charge point's status", event)
	}
}

func TestAcceptedProfileIsReported(t *testing.T) {
	lb, connectors := newTestBalancer(1)
	log := lb.log.(*stubLog)

	connectors[0].CurrentTransactionId = 7
	lb.CheckPowerLimit("chp1")

	log.waitForEvent(t, "accepted")
}

// A charge point that never answers must not be reported as having accepted the
// profile.
func TestUnreadableProfileResponseIsReported(t *testing.T) {
	lb, connectors := newTestBalancer(1)
	lb.server.(*stubServer).payload = `not json`
	log := lb.log.(*stubLog)

	connectors[0].CurrentTransactionId = 7
	lb.CheckPowerLimit("chp1")

	log.waitForEvent(t, "unreadable response")
}

func TestSlotAssignmentSequence(t *testing.T) {
	lb, connectors := newTestBalancer(6)

	repo := lb.database.(*stubRepo)

	// sessions start one by one: highest free slot first, then base limit.
	// Derived from powerSlots so retuning the slots does not make this stale.
	expected := append(append([]int{}, powerSlots...), baseLimit, baseLimit)
	for i, want := range expected {
		connectors[i].CurrentTransactionId = i
		lb.CheckPowerLimit("chp1")
		if got := connectors[i].CurrentPowerLimit; got != want {
			t.Fatalf("connector %d: got %dA, want %dA", i+1, got, want)
		}
		// the assigned limit must also be recorded on the transaction
		if got := repo.txLimits[i]; got != want {
			t.Fatalf("transaction %d: recorded %dA, want %dA", i, got, want)
		}
	}
}

func TestFreedSlotGoesToNextSession(t *testing.T) {
	lb, connectors := newTestBalancer(4)

	for i := range 3 {
		connectors[i].CurrentTransactionId = i
		lb.CheckPowerLimit("chp1")
	}
	// holder of the top slot stops; running sessions keep their limits
	connectors[0].CurrentTransactionId = -1
	lb.CheckPowerLimit("chp1")
	if connectors[0].CurrentPowerLimit != 0 {
		t.Fatalf("stopped connector: got %dA, want 0", connectors[0].CurrentPowerLimit)
	}
	if connectors[1].CurrentPowerLimit != powerSlots[1] || connectors[2].CurrentPowerLimit != powerSlots[2] {
		t.Fatalf("running sessions changed limits: got %dA and %dA, want %dA and %dA",
			connectors[1].CurrentPowerLimit, connectors[2].CurrentPowerLimit, powerSlots[1], powerSlots[2])
	}
	// next new session takes the freed top slot
	connectors[3].CurrentTransactionId = 3
	lb.CheckPowerLimit("chp1")
	if connectors[3].CurrentPowerLimit != powerSlots[0] {
		t.Fatalf("new session: got %dA, want %dA", connectors[3].CurrentPowerLimit, powerSlots[0])
	}
}

func TestBalancingDisabledWithoutLocationLimit(t *testing.T) {
	lb, connectors := newTestBalancer(1)
	repo := lb.database.(*stubRepo)
	repo.location.PowerLimit = 0

	connectors[0].CurrentTransactionId = 0
	lb.CheckPowerLimit("chp1")
	if connectors[0].CurrentPowerLimit != 0 {
		t.Fatalf("got %dA, want no limit when location power limit is 0", connectors[0].CurrentPowerLimit)
	}
}
