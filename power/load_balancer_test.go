package power

import (
	"evsys/entity"
	"evsys/ocpp"
	"testing"
)

type stubRepo struct {
	chargePoint *entity.ChargePoint
	location    *entity.Location
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

type stubServer struct{}

func (s *stubServer) SendRequest(_ string, _ ocpp.Request) (string, error) {
	return "", nil
}

type stubLog struct{}

func (s *stubLog) FeatureEvent(_, _, _ string) {}
func (s *stubLog) RawDataEvent(_, _ string)    {}
func (s *stubLog) Debug(_ string)              {}
func (s *stubLog) Warn(_ string)               {}
func (s *stubLog) Error(_ string, _ error)     {}

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

func TestSlotAssignmentSequence(t *testing.T) {
	lb, connectors := newTestBalancer(6)

	// sessions start one by one: highest free slot first, then base limit
	expected := []int{210, 195, 145, 115, baseLimit, baseLimit}
	for i, want := range expected {
		connectors[i].CurrentTransactionId = i
		lb.CheckPowerLimit("chp1")
		if got := connectors[i].CurrentPowerLimit; got != want {
			t.Fatalf("connector %d: got %dA, want %dA", i+1, got, want)
		}
	}
}

func TestFreedSlotGoesToNextSession(t *testing.T) {
	lb, connectors := newTestBalancer(4)

	for i := range 3 {
		connectors[i].CurrentTransactionId = i
		lb.CheckPowerLimit("chp1")
	}
	// holder of the 210A slot stops; running sessions keep their limits
	connectors[0].CurrentTransactionId = -1
	lb.CheckPowerLimit("chp1")
	if connectors[0].CurrentPowerLimit != 0 {
		t.Fatalf("stopped connector: got %dA, want 0", connectors[0].CurrentPowerLimit)
	}
	if connectors[1].CurrentPowerLimit != 195 || connectors[2].CurrentPowerLimit != 145 {
		t.Fatalf("running sessions changed limits: got %dA and %dA",
			connectors[1].CurrentPowerLimit, connectors[2].CurrentPowerLimit)
	}
	// next new session takes the freed 210A slot
	connectors[3].CurrentTransactionId = 3
	lb.CheckPowerLimit("chp1")
	if connectors[3].CurrentPowerLimit != 210 {
		t.Fatalf("new session: got %dA, want 210A", connectors[3].CurrentPowerLimit)
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
