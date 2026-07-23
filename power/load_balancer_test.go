package power

import (
	"errors"
	"evsys/entity"
	"evsys/ocpp"
	"evsys/ocpp/v16/core"
	"evsys/ocpp/v16/smartcharging"
	"evsys/types"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type stubRepo struct {
	mutex       sync.Mutex
	chargePoint *entity.ChargePoint
	location    *entity.Location
	txLimits    map[int]int                         // transactionId -> last recorded power limit
	verdicts    map[string][]*entity.ProfileVerdict // "chargePointId/connectorId" -> verdicts, in order
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

// Verdicts are written from the goroutine that waits on the charge point, so
// the test's own reads have to be guarded.
func (s *stubRepo) UpdateConnectorProfileVerdict(chargePointId string, connectorId int, verdict *entity.ProfileVerdict) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.verdicts == nil {
		s.verdicts = make(map[string][]*entity.ProfileVerdict)
	}
	key := fmt.Sprintf("%s/%d", chargePointId, connectorId)
	s.verdicts[key] = append(s.verdicts[key], verdict)
	return nil
}

// recordedVerdicts returns a copy of the verdicts stored for a connector.
func (s *stubRepo) recordedVerdicts(chargePointId string, connectorId int) []*entity.ProfileVerdict {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	key := fmt.Sprintf("%s/%d", chargePointId, connectorId)
	return append([]*entity.ProfileVerdict{}, s.verdicts[key]...)
}

// waitForVerdicts blocks until a connector has at least count verdicts recorded.
func (s *stubRepo) waitForVerdicts(t *testing.T, chargePointId string, connectorId, count int) []*entity.ProfileVerdict {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if recorded := s.recordedVerdicts(chargePointId, connectorId); len(recorded) >= count {
			return recorded
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("connector %d recorded %d verdicts, want at least %d",
		connectorId, len(s.recordedVerdicts(chargePointId, connectorId)), count)
	return nil
}

type stubServer struct {
	mutex sync.Mutex
	// payload is the CallResult the stub charge point answers profiles with;
	// empty means it accepts them.
	payload string
	// configPayload is the CallResult it answers GetConfiguration with; empty
	// means the charge point does not answer at all.
	configPayload string
	// silent makes the stub accept profile requests and never answer them, so
	// the balancer falls through to its response timeout.
	silent bool
	// configCalls counts how many times its configuration was read.
	configCalls int
	// profiles records the charging profiles the balancer installed, in order.
	profiles []*types.ChargingProfile
}

func (s *stubServer) SendRequest(_ string, _ ocpp.Request) (string, error) {
	return "", nil
}

func (s *stubServer) SendRequestSync(_ string, request ocpp.Request, _ time.Duration) (string, error) {
	if _, ok := request.(*core.GetConfigurationRequest); ok {
		s.mutex.Lock()
		defer s.mutex.Unlock()
		s.configCalls++
		if s.configPayload == "" {
			return "", errors.New("no response")
		}
		return s.configPayload, nil
	}
	return `{"status":"Accepted"}`, nil
}

func (s *stubServer) configurationReads() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.configCalls
}

// SendRequestWithResponse answers immediately so the verdict goroutine the load
// balancer spawns finishes within the test rather than sitting on its timeout.
func (s *stubServer) SendRequestWithResponse(_ string, request ocpp.Request) (<-chan string, func(), error) {
	s.mutex.Lock()
	payload := s.payload
	silent := s.silent
	if profile, ok := request.(*smartcharging.SetChargingProfileRequest); ok {
		s.profiles = append(s.profiles, profile.ChargingProfile)
	}
	s.mutex.Unlock()

	response := make(chan string, 1)
	if silent {
		// queued but never answered
		return response, func() {}, nil
	}
	if payload == "" {
		payload = `{"status":"Accepted"}`
	}
	response <- payload
	return response, func() {}, nil
}

// installedProfiles returns a copy, since the balancer writes to the slice from
// its verdict goroutines.
func (s *stubServer) installedProfiles() []*types.ChargingProfile {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return append([]*types.ChargingProfile{}, s.profiles...)
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

// current_power_limit records what was asked for and is written before the
// charge point answers, so on its own it cannot tell a limit in force from one
// that was refused. The verdict has to be stored for that question to be
// answerable without reading the log.
func TestProfileVerdictIsRecorded(t *testing.T) {
	tests := []struct {
		name     string
		payload  string
		expected string
	}{
		{name: "accepted", payload: `{"status":"Accepted"}`, expected: entity.ProfileStatusAccepted},
		{name: "rejected", payload: `{"status":"Rejected"}`, expected: entity.ProfileStatusRejected},
		{name: "not supported", payload: `{"status":"NotSupported"}`, expected: entity.ProfileStatusNotSupported},
		{name: "unreadable answer", payload: `not json`, expected: entity.ProfileStatusUnreadable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lb, connectors := newTestBalancer(1)
			lb.server.(*stubServer).payload = test.payload
			repo := lb.database.(*stubRepo)

			connectors[0].CurrentTransactionId = 7
			lb.CheckPowerLimit("chp1")

			verdict := repo.waitForVerdicts(t, "chp1", 1, 1)[0]
			if verdict.Status != test.expected {
				t.Errorf("status = %q, want %q", verdict.Status, test.expected)
			}
			if verdict.Limit != powerSlots[0] {
				t.Errorf("limit = %d, want %d", verdict.Limit, powerSlots[0])
			}
			if verdict.StackLevel != smartcharging.TxProfileStackLevel {
				t.Errorf("stack level = %d, want %d", verdict.StackLevel, smartcharging.TxProfileStackLevel)
			}
			if verdict.Time.IsZero() {
				t.Error("verdict has no timestamp; an old refusal would be indistinguishable from a current one")
			}
		})
	}
}

// The stack-level retry must leave behind the outcome of the attempt that
// actually settled, not the one that was superseded.
func TestRetryRecordsBothAttempts(t *testing.T) {
	lb, connectors := newTestBalancer(1)
	server := lb.server.(*stubServer)
	server.configPayload = configResponse("10", "Current")
	server.payload = `{"status":"Rejected"}`
	repo := lb.database.(*stubRepo)

	lb.onChargePointBoot("chp1")
	connectors[0].CurrentTransactionId = 7
	lb.CheckPowerLimit("chp1")

	verdicts := repo.waitForVerdicts(t, "chp1", 1, 2)
	if verdicts[0].StackLevel != 10 || verdicts[1].StackLevel != 9 {
		t.Fatalf("recorded stack levels %d then %d, want 10 then 9",
			verdicts[0].StackLevel, verdicts[1].StackLevel)
	}
}

// Silence is not a refusal: the charge point never weighed in, so a lower stack
// level is no more likely to land and retrying would only overwrite the record
// of what actually happened.
func TestSilenceIsRecordedAndNotRetried(t *testing.T) {
	lb, connectors := newTestBalancer(1)
	server := lb.server.(*stubServer)
	server.configPayload = configResponse("10", "Current")
	repo := lb.database.(*stubRepo)
	// the real wait is 30s; the path under test is what happens when it expires
	lb.profileTimeout = 10 * time.Millisecond

	lb.onChargePointBoot("chp1")
	server.mutex.Lock()
	server.silent = true
	server.mutex.Unlock()

	connectors[0].CurrentTransactionId = 7
	lb.CheckPowerLimit("chp1")

	verdicts := repo.waitForVerdicts(t, "chp1", 1, 1)
	if verdicts[0].Status != entity.ProfileStatusNoResponse {
		t.Fatalf("status = %q, want %q", verdicts[0].Status, entity.ProfileStatusNoResponse)
	}
	if got := lb.capabilities.get("chp1").maxStackLevel; got != 10 {
		t.Fatalf("stack level lowered to %d; only an explicit refusal should lower it", got)
	}
	// and the profile was not resent at a lower level
	time.Sleep(50 * time.Millisecond)
	if recorded := repo.recordedVerdicts("chp1", 1); len(recorded) != 1 {
		t.Fatalf("recorded %d verdicts, want 1: silence was retried", len(recorded))
	}
}

func TestVerdictAcceptedHelper(t *testing.T) {
	var missing *entity.ProfileVerdict
	if missing.Accepted() {
		t.Error("a connector that was never sent a profile reads as enforced")
	}
	if (&entity.ProfileVerdict{Status: entity.ProfileStatusRejected}).Accepted() {
		t.Error("a refused profile reads as enforced")
	}
	if !(&entity.ProfileVerdict{Status: entity.ProfileStatusAccepted}).Accepted() {
		t.Error("an accepted profile does not read as enforced")
	}
}

func configResponse(maxStackLevel, allowedUnits string) string {
	return `{"configurationKey":[` +
		`{"key":"ChargeProfileMaxStackLevel","readonly":true,"value":"` + maxStackLevel + `"},` +
		`{"key":"ChargingScheduleAllowedChargingRateUnit","readonly":true,"value":"` + allowedUnits + `"}]}`
}

// A charge point that caps the stack level below ours must not be sent profiles
// it will refuse.
func TestStackLevelClampedToReportedMaximum(t *testing.T) {
	lb, connectors := newTestBalancer(1)
	server := lb.server.(*stubServer)
	server.configPayload = configResponse("3", "Current")
	log := lb.log.(*stubLog)

	lb.onChargePointBoot("chp1")
	log.waitForEvent(t, "ChargeProfileMaxStackLevel=3")

	connectors[0].CurrentTransactionId = 7
	lb.CheckPowerLimit("chp1")
	log.waitForEvent(t, "at stack level 3")

	for _, profile := range server.installedProfiles() {
		if profile.ChargingProfilePurpose != types.ChargingProfilePurposeTxProfile {
			continue
		}
		if profile.StackLevel > 3 {
			t.Fatalf("installed stack level %d, above the reported maximum of 3", profile.StackLevel)
		}
	}
}

// OCPP 1.6 does not say whether ChargeProfileMaxStackLevel is inclusive, so a
// charge point reporting 10 may accept only 0..9. A rejection must be retried a
// level lower rather than leaving the session unlimited.
func TestRejectedProfileRetriesALevelLower(t *testing.T) {
	lb, connectors := newTestBalancer(1)
	server := lb.server.(*stubServer)
	server.configPayload = configResponse("10", "Current")
	server.payload = `{"status":"Rejected"}`
	log := lb.log.(*stubLog)

	lb.onChargePointBoot("chp1")
	log.waitForEvent(t, "ChargeProfileMaxStackLevel=10")

	connectors[0].CurrentTransactionId = 7
	lb.CheckPowerLimit("chp1")
	log.waitForEvent(t, "at stack level 9")

	levels := map[int]bool{}
	for _, profile := range server.installedProfiles() {
		if profile.ChargingProfilePurpose == types.ChargingProfilePurposeTxProfile {
			levels[profile.StackLevel] = true
		}
	}
	if !levels[10] || !levels[9] {
		t.Fatalf("expected attempts at stack levels 10 and 9, got %v", levels)
	}
	// one step down only: a charge point rejecting for some other reason must
	// not make the balancer walk the whole range
	if levels[8] {
		t.Fatalf("retried more than one level down: %v", levels)
	}
}

// The retry must remember the level that worked, so the next session starts
// there instead of taking the rejection again.
func TestNegotiatedStackLevelIsRemembered(t *testing.T) {
	lb, connectors := newTestBalancer(2)
	server := lb.server.(*stubServer)
	server.configPayload = configResponse("10", "Current")
	server.payload = `{"status":"Rejected"}`
	log := lb.log.(*stubLog)

	lb.onChargePointBoot("chp1")
	log.waitForEvent(t, "ChargeProfileMaxStackLevel=10")

	connectors[0].CurrentTransactionId = 7
	lb.CheckPowerLimit("chp1")
	log.waitForEvent(t, "at stack level 9")

	if got := lb.capabilities.get("chp1").maxStackLevel; got != 9 {
		t.Fatalf("remembered stack level %d, want 9", got)
	}
}

// A charge point that accepts only Power schedules cannot be given an amperage
// limit; saying so beats sending a profile that will be refused.
func TestPowerOnlyChargePointIsReported(t *testing.T) {
	lb, connectors := newTestBalancer(1)
	server := lb.server.(*stubServer)
	server.configPayload = configResponse("10", "Power")
	log := lb.log.(*stubLog)

	lb.onChargePointBoot("chp1")
	log.waitForEvent(t, "CANNOT ENFORCE POWER LIMITS")

	connectors[0].CurrentTransactionId = 7
	lb.CheckPowerLimit("chp1")
	log.waitForEvent(t, "accepts Power schedules only")

	for _, profile := range server.installedProfiles() {
		if profile.ChargingProfilePurpose == types.ChargingProfilePurposeTxProfile {
			t.Fatal("installed a transaction profile the charge point cannot honour")
		}
	}
}

// A charge point that does not answer the configuration read must keep working
// on the protocol defaults rather than losing its limits.
func TestUnknownCapabilitiesFallBackToDefaults(t *testing.T) {
	lb, connectors := newTestBalancer(1)
	log := lb.log.(*stubLog)

	lb.onChargePointBoot("chp1") // stub answers no configuration
	log.waitForEvent(t, "using defaults")

	connectors[0].CurrentTransactionId = 7
	lb.CheckPowerLimit("chp1")

	if got := connectors[0].CurrentPowerLimit; got != powerSlots[0] {
		t.Fatalf("got %dA, want %dA", got, powerSlots[0])
	}
	log.waitForEvent(t, fmt.Sprintf("at stack level %d", smartcharging.TxProfileStackLevel))
}

// Nothing about a charge point's configuration survives a restart of the
// central system, and a charge point that stays up sends no BootNotification to
// prompt a fresh read. Its limits therefore have to be discovered on first use,
// or a restart would leave every charge point on the fallback defaults forever.
func TestCapabilitiesDiscoveredWithoutBootNotification(t *testing.T) {
	lb, connectors := newTestBalancer(1)
	server := lb.server.(*stubServer)
	server.configPayload = configResponse("3", "Current")
	log := lb.log.(*stubLog)

	// no onChargePointBoot: the charge point was already connected
	connectors[0].CurrentTransactionId = 7
	lb.CheckPowerLimit("chp1")

	log.waitForEvent(t, "ChargeProfileMaxStackLevel=3")
	log.waitForEvent(t, "at stack level 3")
	if got := server.configurationReads(); got != 1 {
		t.Fatalf("read the configuration %d times, want 1", got)
	}
}

// Once read, the configuration is not read again on every session event.
func TestCapabilitiesReadOnlyOnce(t *testing.T) {
	lb, connectors := newTestBalancer(2)
	server := lb.server.(*stubServer)
	server.configPayload = configResponse("10", "Current")

	for i := range connectors {
		connectors[i].CurrentTransactionId = i
		lb.CheckPowerLimit("chp1")
	}
	lb.CheckPowerLimit("chp1")

	if got := server.configurationReads(); got != 1 {
		t.Fatalf("read the configuration %d times, want 1", got)
	}
}

// A charge point that does not answer must not be asked again on every start and
// stop: each attempt costs a full timeout.
func TestMuteChargePointIsNotReaskedImmediately(t *testing.T) {
	lb, connectors := newTestBalancer(2)
	server := lb.server.(*stubServer) // no configPayload: never answers

	for i := range connectors {
		connectors[i].CurrentTransactionId = i
		lb.CheckPowerLimit("chp1")
	}

	if got := server.configurationReads(); got != 1 {
		t.Fatalf("asked a mute charge point %d times, want 1 within the retry interval", got)
	}
	// and it is asked again once the interval has passed
	if !lb.capabilities.beginDiscovery("chp1", time.Now().Add(discoveryRetryInterval+time.Second), discoveryRetryInterval) {
		t.Fatal("mute charge point was never retried")
	}
}

func TestCapabilityStoreLearnsCeilingWithoutBlockingDiscovery(t *testing.T) {
	now := time.Now()

	t.Run("a rejection does not count as having read the configuration", func(t *testing.T) {
		store := newCapabilityStore()
		store.lowerStackLevel("chp1", 9)

		if !store.beginDiscovery("chp1", now, discoveryRetryInterval) {
			t.Fatal("a rejected profile blocked the configuration read; a charge point " +
				"that refuses everything would never be asked what it accepts")
		}
		if got := store.get("chp1").stackLevelFor(smartcharging.TxProfileStackLevel); got != 9 {
			t.Fatalf("stack level %d, want the learned ceiling 9", got)
		}
	})

	t.Run("a learned ceiling survives a later configuration read", func(t *testing.T) {
		store := newCapabilityStore()
		store.lowerStackLevel("chp1", 9)

		reported, err := parseCapabilities(configResponse("10", "Current"))
		if err != nil {
			t.Fatalf("parseCapabilities: %v", err)
		}
		stored := store.record("chp1", reported)

		if stored.maxStackLevel != 9 {
			t.Fatalf("stack level %d, want 9: the charge point reports 10 every time, "+
				"so discarding what its refusal taught us re-pays the rejection", stored.maxStackLevel)
		}
		if !stored.allowsCurrent {
			t.Fatal("the reported rate unit was lost")
		}
	})

	t.Run("a higher learned ceiling does not override a lower report", func(t *testing.T) {
		store := newCapabilityStore()
		store.lowerStackLevel("chp1", 9)

		reported, err := parseCapabilities(configResponse("3", "Current"))
		if err != nil {
			t.Fatalf("parseCapabilities: %v", err)
		}
		if got := store.record("chp1", reported).maxStackLevel; got != 3 {
			t.Fatalf("stack level %d, want the reported 3", got)
		}
	})
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
