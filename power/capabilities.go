package power

import (
	"encoding/json"
	"evsys/ocpp/v16/core"
	"evsys/ocpp/v16/smartcharging"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// keyMaxStackLevel is the highest stack level a charging profile may use.
	// OCPP 1.6 describes it as "Max StackLevel of a ChargingProfile" and, in the
	// same sentence, as the number of schedules allowed per purpose - so a
	// charge point reporting 10 may accept levels 0..10 or only 0..9. We ask for
	// the reported value and fall back a level if it is rejected.
	keyMaxStackLevel = "ChargeProfileMaxStackLevel"
	// keyAllowedRateUnit is a comma-separated list of "Current" and/or "Power".
	// A charge point that allows only Power cannot be given an amperage limit.
	keyAllowedRateUnit = "ChargingScheduleAllowedChargingRateUnit"

	rateUnitCurrent = "Current"

	// capabilityTimeout bounds a configuration read. At boot this runs on its own
	// goroutine; on the lazy path it runs before the balancer lock is taken, so
	// in neither case does it hold up another session.
	capabilityTimeout = 15 * time.Second

	// discoveryRetryInterval is how long to leave a charge point alone after it
	// failed to report its configuration. Without it, every start and stop at a
	// location with one mute charge point would pay capabilityTimeout again.
	discoveryRetryInterval = 5 * time.Minute
)

// capabilities is what a charge point reported about its smart charging limits.
// The zero value means we have not asked, or the charge point did not answer;
// callers then fall back to the protocol defaults.
type capabilities struct {
	// known reports whether the charge point's configuration has been read. It
	// gates the rate unit and whether discovery still needs to run; the stack
	// level ceiling is tracked separately, because a rejected profile teaches us
	// a ceiling without telling us anything else.
	known bool
	// stackLevelKnown reports whether maxStackLevel means anything.
	stackLevelKnown bool
	// maxStackLevel is the charge point's reported ChargeProfileMaxStackLevel,
	// lowered whenever it rejects a profile at the level we tried.
	maxStackLevel int
	// allowsCurrent reports whether amperage-based schedules are accepted. Only
	// meaningful while known: a rejection says nothing about the rate unit.
	allowsCurrent bool
	// allowedUnits is the raw reported list, kept for logging.
	allowedUnits string
	// lastAttempt is when we last asked. Zero means never; it is only consulted
	// while known is false, to keep a charge point that does not answer from
	// being asked on every session event.
	lastAttempt time.Time
}

// stackLevelFor clamps the level we want to what the charge point accepts.
func (c capabilities) stackLevelFor(desired int) int {
	if !c.stackLevelKnown || c.maxStackLevel >= desired {
		return desired
	}
	if c.maxStackLevel < 0 {
		return 0
	}
	return c.maxStackLevel
}

// capabilityStore caches per-charge-point smart charging limits. It has its own
// mutex: entries are written from the boot goroutine and from the goroutines
// that read charging profile verdicts, neither of which holds the balancer lock.
type capabilityStore struct {
	mutex   sync.Mutex
	entries map[string]capabilities
}

func newCapabilityStore() *capabilityStore {
	return &capabilityStore{entries: make(map[string]capabilities)}
}

func (s *capabilityStore) get(chargePointId string) capabilities {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.entries[chargePointId]
}

// record stores a freshly read configuration. A ceiling already learned from a
// rejection is kept if it is lower than the reported one: the charge point
// reports the same number every time, and discarding what its own refusals
// taught us would make every restart re-pay that rejection.
func (s *capabilityStore) record(chargePointId string, reported capabilities) capabilities {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	previous := s.entries[chargePointId]
	if previous.stackLevelKnown && previous.maxStackLevel < reported.maxStackLevel {
		reported.maxStackLevel = previous.maxStackLevel
	}
	s.entries[chargePointId] = reported
	return reported
}

// beginDiscovery claims the right to read a charge point's configuration,
// reporting false when it is already known or was attempted too recently. The
// attempt is recorded before the read runs, so two sessions starting at once
// produce one read rather than two, and the loser proceeds on the fallback
// defaults instead of waiting on the network.
func (s *capabilityStore) beginDiscovery(chargePointId string, now time.Time, retryInterval time.Duration) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	entry := s.entries[chargePointId]
	if entry.known {
		return false
	}
	if !entry.lastAttempt.IsZero() && now.Sub(entry.lastAttempt) < retryInterval {
		return false
	}
	entry.lastAttempt = now
	s.entries[chargePointId] = entry
	return true
}

// lowerStackLevel records that the charge point refused a profile at the level
// we tried, so later profiles start below it. Reports whether anything changed.
func (s *capabilityStore) lowerStackLevel(chargePointId string, level int) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	entry := s.entries[chargePointId]
	if entry.stackLevelKnown && entry.maxStackLevel <= level {
		return false
	}
	// Only the ceiling is learned here. A rejection does not say why, so it must
	// not be read as the charge point having reported its configuration -
	// otherwise a charge point that refuses every profile would never be asked
	// what it actually accepts.
	entry.stackLevelKnown = true
	entry.maxStackLevel = level
	s.entries[chargePointId] = entry
	return true
}

// parseCapabilities reads a GetConfiguration response. Keys the charge point
// does not know are simply absent, leaving their fields at the fallback.
func parseCapabilities(payload string) (capabilities, error) {
	var response core.GetConfigurationResponse
	if err := json.Unmarshal([]byte(payload), &response); err != nil {
		return capabilities{}, fmt.Errorf("unreadable configuration response %q: %w", payload, err)
	}
	result := capabilities{
		known:           true,
		stackLevelKnown: true,
		// Absent keys must not silently disable limiting: assume the protocol
		// default until the charge point says otherwise.
		maxStackLevel: smartcharging.TxProfileStackLevel,
		allowsCurrent: true,
		allowedUnits:  rateUnitCurrent,
	}
	for _, key := range response.ConfigurationKey {
		if key.Value == nil {
			continue
		}
		switch key.Key {
		case keyMaxStackLevel:
			level, err := strconv.Atoi(strings.TrimSpace(*key.Value))
			if err != nil {
				return capabilities{}, fmt.Errorf("%s is not a number: %q", keyMaxStackLevel, *key.Value)
			}
			result.maxStackLevel = level
		case keyAllowedRateUnit:
			result.allowedUnits = *key.Value
			result.allowsCurrent = false
			for _, unit := range strings.Split(*key.Value, ",") {
				if strings.EqualFold(strings.TrimSpace(unit), rateUnitCurrent) {
					result.allowsCurrent = true
				}
			}
		}
	}
	return result, nil
}
