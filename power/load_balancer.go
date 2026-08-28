package power

import (
	"encoding/json"
	"evsys/entity"
	"evsys/internal"
	"evsys/ocpp"
	"evsys/ocpp/v16/core"
	"evsys/ocpp/v16/smartcharging"
	"fmt"
	"sync"
	"time"
)

const (
	featureName = "LoadBalancer"
	baseLimit   = 85
	// profileResponseTimeout bounds how long we wait for a charge point to
	// confirm a charging profile. The wait always runs on its own goroutine, so
	// it delays nothing; it only has to be generous enough that a slow link is
	// not reported as silence.
	profileResponseTimeout = 30 * time.Second
	// stackLevelRetries is how many times a rejected profile is retried a stack
	// level lower. One step is enough for the off-by-one in the OCPP 1.6
	// definition of ChargeProfileMaxStackLevel; more would just walk the whole
	// range against a charge point rejecting for some other reason.
	stackLevelRetries = 1
)

// profileStatus is the shape shared by the SetChargingProfile and
// ClearChargingProfile responses. Both report "Accepted" on success.
type profileStatus struct {
	Status string `json:"status"`
}

const profileAccepted = "Accepted"

// powerSlots are assigned highest-first, one connector per slot;
// when all slots are taken, further connectors get baseLimit
var powerSlots = []int{150, 115, 85}

type LoadBalancer struct {
	database     Repository
	server       Handler
	log          internal.LogHandler
	capabilities *capabilityStore
	// profileTimeout is how long to wait for a charge point to rule on a
	// profile. A field rather than a constant so a test can drive the
	// no-response path without waiting out the real one.
	profileTimeout time.Duration
	mutex          sync.Mutex
}

func NewLoadBalancer(database Repository, server Handler, log internal.LogHandler) *LoadBalancer {
	return &LoadBalancer{
		database:       database,
		server:         server,
		log:            log,
		capabilities:   newCapabilityStore(),
		profileTimeout: profileResponseTimeout,
		mutex:          sync.Mutex{},
	}
}

// OnChargePointBoot runs on its own goroutine: it reads the charge point's smart
// charging configuration, which is a round trip and must not hold up the
// connection's read pump.
func (lb *LoadBalancer) OnChargePointBoot(chargePointId string) {
	go lb.onChargePointBoot(chargePointId)
}

func (lb *LoadBalancer) onChargePointBoot(chargePointId string) {
	// getLocation only reads fields fixed at construction, so it is safe to call
	// before taking the balancer lock - and discovery must not hold that lock
	// while it waits on the network.
	location, _ := lb.getLocation(chargePointId)
	if location == nil {
		return
	}
	lb.discoverCapabilities(chargePointId)

	lb.mutex.Lock()
	defer lb.mutex.Unlock()
	var request ocpp.Request
	var description string
	if location.DefaultPowerLimit == 0 {
		description = "clearing default charging profile"
		request = smartcharging.NewClearDefaultChargingProfileRequest()
	} else {
		description = fmt.Sprintf("setting default charging profile to %dA", location.DefaultPowerLimit)
		request = smartcharging.NewSetChargingProfileRequest(0, smartcharging.NewDefaultChargingProfile(location.DefaultPowerLimit))
	}
	lb.log.FeatureEvent(featureName, chargePointId, description)
	if err := lb.sendProfile(chargePointId, description, request, nil); err != nil {
		lb.log.FeatureEvent(featureName, chargePointId, fmt.Sprintf("error sending request: %s", err))
	}
}

// ensureCapabilities reads a charge point's smart charging limits if we do not
// already have them. Nothing is persisted between runs of the central system,
// and a charge point only reports its configuration when asked, so without this
// a restart would leave every charge point on the fallback defaults until it
// next rebooted - which, for a charge point that stays up, may be never.
//
// A charge point that does not answer is not asked again for
// discoveryRetryInterval, so a mute charger costs one timeout rather than one
// per session event.
func (lb *LoadBalancer) ensureCapabilities(chargePointId string) {
	if !lb.capabilities.beginDiscovery(chargePointId, time.Now(), discoveryRetryInterval) {
		return
	}
	lb.discoverCapabilities(chargePointId)
}

// discoverCapabilities asks the charge point what charging profiles it will
// accept, so we stop guessing at the stack level and the rate unit.
func (lb *LoadBalancer) discoverCapabilities(chargePointId string) {
	request := core.NewGetConfigurationRequest([]string{keyMaxStackLevel, keyAllowedRateUnit})
	payload, err := lb.server.SendRequestSync(chargePointId, request, capabilityTimeout)
	if err != nil {
		lb.log.FeatureEvent(featureName, chargePointId,
			fmt.Sprintf("smart charging configuration unavailable, using defaults: %s", err))
		return
	}
	reported, err := parseCapabilities(payload)
	if err != nil {
		lb.log.FeatureEvent(featureName, chargePointId, fmt.Sprintf("smart charging configuration: %s", err))
		return
	}
	stored := lb.capabilities.record(chargePointId, reported)
	lb.log.FeatureEvent(featureName, chargePointId, fmt.Sprintf(
		"smart charging configuration: %s=%d (using %d), %s=%s",
		keyMaxStackLevel, reported.maxStackLevel, stored.maxStackLevel,
		keyAllowedRateUnit, reported.allowedUnits))
	if !reported.allowsCurrent {
		lb.log.FeatureEvent(featureName, chargePointId, fmt.Sprintf(
			"CANNOT ENFORCE POWER LIMITS: charge point accepts %s schedules only, and the balancer works in amperes",
			reported.allowedUnits))
	}
}

// sendProfile queues a charging profile request and logs the charge point's
// verdict once it arrives. A charge point that rejects a profile keeps charging
// at whatever its own hardware allows, which looks identical to a limit
// correctly applied unless the response is read.
//
// onVerdict, when set, runs on the same goroutine once the outcome is known -
// including when no usable answer arrived, which is as much a failure to
// enforce a limit as an outright refusal.
func (lb *LoadBalancer) sendProfile(chargePointId, description string, request ocpp.Request, onVerdict func(status string)) error {
	response, release, err := lb.server.SendRequestWithResponse(chargePointId, request)
	if err != nil {
		return err
	}
	go func() {
		defer release()
		verdict := entity.ProfileStatusNoResponse
		select {
		case payload := <-response:
			var status profileStatus
			if err := json.Unmarshal([]byte(payload), &status); err != nil {
				verdict = entity.ProfileStatusUnreadable
				lb.log.FeatureEvent(featureName, chargePointId,
					fmt.Sprintf("%s: unreadable response: %s", description, payload))
				break
			}
			verdict = status.Status
			if verdict != profileAccepted {
				lb.log.FeatureEvent(featureName, chargePointId,
					fmt.Sprintf("%s: REJECTED by charge point, status %s", description, verdict))
				break
			}
			lb.log.FeatureEvent(featureName, chargePointId, fmt.Sprintf("%s: accepted", description))
		case <-time.After(lb.profileTimeout):
			lb.log.FeatureEvent(featureName, chargePointId,
				fmt.Sprintf("%s: no response within %s", description, lb.profileTimeout))
		}
		if onVerdict != nil {
			onVerdict(verdict)
		}
	}()
	return nil
}

// recordVerdict stores what the charge point said about a limit, so the
// question "is this limit in force" is a database read rather than a log grep.
// The connector document is updated by identity and the in-memory connector is
// left alone: this runs after the balancer lock has been released, and the next
// pass reads the connector fresh from the database anyway.
func (lb *LoadBalancer) recordVerdict(connector *entity.Connector, status string, powerLimit, stackLevel int) {
	if lb.database == nil {
		return
	}
	verdict := &entity.ProfileVerdict{
		Status:     status,
		Limit:      powerLimit,
		StackLevel: stackLevel,
		Time:       time.Now(),
	}
	if err := lb.database.UpdateConnectorProfileVerdict(connector.ChargePointId, connector.Id, verdict); err != nil {
		lb.log.FeatureEvent(featureName, connector.ChargePointId,
			fmt.Sprintf("error recording profile verdict: %s", err))
	}
}

// sendTransactionProfile installs a session's power limit at the stack level the
// charge point is currently believed to accept.
func (lb *LoadBalancer) sendTransactionProfile(connector *entity.Connector, powerLimit int, connectorInfo string) error {
	limits := lb.capabilities.get(connector.ChargePointId)
	if limits.known && !limits.allowsCurrent {
		return fmt.Errorf("charge point accepts %s schedules only", limits.allowedUnits)
	}
	return lb.sendTransactionProfileAt(connector, powerLimit, connectorInfo,
		limits.stackLevelFor(smartcharging.TxProfileStackLevel), stackLevelRetries)
}

// sendTransactionProfileAt installs the limit at an explicit stack level, stepping
// down and retrying if the charge point refuses. OCPP 1.6 defines
// ChargeProfileMaxStackLevel both as the highest usable level and as the number
// of levels available, so a charge point reporting 10 may accept only 0..9. The
// retry settles that against the hardware instead of guessing.
//
// The retry and the remembered ceiling are deliberately separate. Retrying is
// this session's business and always worth one step - a session that takes no
// limit charges at whatever the hardware allows. Lowering the ceiling is a claim
// about the charge point that outlives the session, and a refusal at a level
// that has worked before is no evidence for it; the store decides that, not the
// retry. The level is passed in rather than re-read for exactly that reason: the
// store is entitled to refuse the demotion the retry is about to act on.
func (lb *LoadBalancer) sendTransactionProfileAt(connector *entity.Connector, powerLimit int, connectorInfo string, stackLevel, retriesLeft int) error {
	chargePointId := connector.ChargePointId
	transactionId := connector.CurrentTransactionId

	description := fmt.Sprintf("power limit %dA for %s at stack level %d", powerLimit, connectorInfo, stackLevel)
	lb.log.FeatureEvent(featureName, chargePointId, "setting "+description)

	request := smartcharging.NewSetChargingProfileRequest(
		connector.Id, smartcharging.NewTransactionChargingProfile(
			connector.Id, transactionId, powerLimit, stackLevel))

	return lb.sendProfile(chargePointId, description, request, func(status string) {
		lb.recordVerdict(connector, status, powerLimit, stackLevel)
		if status == entity.ProfileStatusAccepted {
			lb.capabilities.recordAccepted(chargePointId, stackLevel)
			return
		}
		// Only an outright refusal is worth stepping down for. Silence means the
		// charge point never weighed in, so a lower level would be no more
		// likely to land and would only overwrite the record of what happened.
		if status != entity.ProfileStatusRejected && status != entity.ProfileStatusNotSupported {
			return
		}
		if stackLevel <= 0 {
			// every level has been tried and refused, so the refusals were never
			// about the stack level and this connector has no limit in force
			lb.log.FeatureEvent(featureName, chargePointId, fmt.Sprintf(
				"CANNOT ENFORCE POWER LIMITS: the profile for %s was refused at every stack level down to 0",
				connectorInfo))
			return
		}
		if retriesLeft <= 0 {
			return
		}
		if lb.capabilities.lowerStackLevel(chargePointId, stackLevel-1) {
			lb.log.FeatureEvent(featureName, chargePointId, fmt.Sprintf(
				"stack level ceiling lowered to %d after refusal at %d", stackLevel-1, stackLevel))
		} else {
			lb.log.FeatureEvent(featureName, chargePointId, fmt.Sprintf(
				"refusal at stack level %d is not a new ceiling; keeping %d for later sessions",
				stackLevel, lb.capabilities.get(chargePointId).stackLevelFor(smartcharging.TxProfileStackLevel)))
		}
		// The connector is read again rather than captured: retrying a limit for
		// a session that has since stopped would install a profile for a
		// transaction the charge point no longer has.
		if connector.CurrentTransactionId != transactionId {
			return
		}
		if err := lb.sendTransactionProfileAt(connector, powerLimit, connectorInfo, stackLevel-1, retriesLeft-1); err != nil {
			lb.log.FeatureEvent(featureName, chargePointId,
				fmt.Sprintf("retry at stack level %d failed: %s", stackLevel-1, err))
		}
	})
}

func (lb *LoadBalancer) OnSystemStart() {
	if lb.database == nil {
		return
	}
	locations, err := lb.database.GetLocations()
	if err != nil {
		lb.log.FeatureEvent(featureName, "", fmt.Sprintf("error getting locations: %s", err))
		return
	}
	// check all connectors and reset power limit in database
	for _, location := range locations {
		chargers := 0
		connectors := 0
		for _, chp := range location.Evses {
			if chp.SmartCharging {
				chargers++
				for _, connector := range chp.Connectors {
					connectors++
					if connector.CurrentTransactionId < 0 && connector.CurrentPowerLimit != 0 {
						connector.CurrentPowerLimit = 0
						err = lb.database.UpdateConnectorCurrentPower(connector)
						if err != nil {
							lb.log.FeatureEvent(featureName, "", fmt.Sprintf("database error: %s", err))
						}
					}
				}
			}
		}
		lb.log.FeatureEvent(featureName, location.Id, fmt.Sprintf("location %s: %d chargers, %d connectors", location.Name, chargers, connectors))
	}
}

// isBalanced reports whether the balancer may install profiles on this charge
// point. It reads only configuration, never session state, which is what makes it
// safe to call before the balancer lock.
//
// A charge point that cannot be read reports true. The answer is unknown, and
// getLocation under the lock is where that is logged and decided; answering false
// here would swallow the diagnostic.
func (lb *LoadBalancer) isBalanced(chargePointId string) bool {
	if lb.database == nil {
		return false
	}
	chp, err := lb.database.GetChargePoint(chargePointId)
	if err != nil || chp == nil {
		return true
	}
	return chp.SmartCharging
}

func (lb *LoadBalancer) CheckPowerLimit(chargePointId string) {
	// Capabilities are read over the network, and a charge point that boots while
	// the central system is down never sends the BootNotification that would
	// otherwise have supplied them. Discovering here means every charge point
	// learns its own limits on its first session after a restart, whether or not
	// it announced itself - and before the lock, so the wait blocks nobody.
	//
	// Only for a charge point that will actually be sent profiles, though. Asking
	// one that is not smart charging buys a round trip it can never benefit from,
	// and a full capabilityTimeout every discoveryRetryInterval when it does not
	// answer. Every start and stop on the unbalanced half of this fleet was paying
	// for a configuration read of a charge point the balancer never touches.
	if !lb.isBalanced(chargePointId) {
		return
	}
	lb.ensureCapabilities(chargePointId)

	lb.mutex.Lock()
	defer lb.mutex.Unlock()
	// The location is read under the lock rather than alongside the charge point
	// above: its connectors carry the session state the slot assignment is
	// computed from, and reading them outside would let two sessions starting at
	// once both see the same slot free and take it.
	location, _ := lb.getLocation(chargePointId)
	if location == nil {
		return
	}
	usedSlots := make(map[int]bool)
	// all active connectors on smart charging points
	activeConnectors := 0
	for _, chp := range location.Evses {
		if chp.SmartCharging {
			for _, connector := range chp.Connectors {
				if connector.CurrentTransactionId >= 0 {
					activeConnectors++
					usedSlots[connector.CurrentPowerLimit] = true
				} else if connector.CurrentPowerLimit > 0 {
					// clear power limit for connector with no active transaction
					err := lb.updateConnectorPower(0, connector)
					if err != nil {
						lb.log.FeatureEvent(featureName, chargePointId, fmt.Sprintf("error updating connector: %s", err))
					}
				}
			}
		}
	}
	if activeConnectors == 0 {
		return
	}
	powerLimit := baseLimit
	for _, slot := range powerSlots {
		if !usedSlots[slot] {
			powerLimit = slot
			break
		}
	}
	lb.log.FeatureEvent(featureName, chargePointId, fmt.Sprintf("active connectors: %d; assigning %dA to new sessions", activeConnectors, powerLimit))

	// send set charging profile request to each active connector
	for _, chp := range location.Evses {
		if !chp.SmartCharging {
			continue
		}
		for _, connector := range chp.Connectors {
			assigned := powerLimit
			if connector.CurrentTransactionId >= 0 && connector.CurrentPowerLimit > 0 {
				if !connector.LimitRefused() {
					continue
				}
				// CurrentPowerLimit is written when the profile is queued, so a
				// connector whose profile was refused looks identical to one
				// charging under a limit - and the slot it books is counted
				// above either way. Reinstalling is what makes the record true
				// again; until it lands this connector is a hole in the budget
				// that no amount of restraint elsewhere closes.
				//
				// The slot is already this session's, so the reattempt asks for
				// that limit rather than the one computed for new sessions. A
				// refusal is often transient - a charge point busy with its own
				// profile store, a link that dropped mid-request - so the next
				// session event at this location is the natural moment to try
				// again, and the verdict recorded for the new attempt decides
				// whether there is a next one.
				assigned = connector.CurrentPowerLimit
				lb.log.FeatureEvent(featureName, connector.ChargePointId, fmt.Sprintf(
					"connector %d has no limit in force: %dA answered with %s; reinstalling",
					connector.Id, connector.CurrentPowerLimit, connector.LastProfile.Answer()))
			}
			if err := lb.updateConnectorPower(assigned, connector); err != nil {
				lb.log.FeatureEvent(featureName, chargePointId, fmt.Sprintf("error updating connector: %s", err))
			}
		}
	}
}

func (lb *LoadBalancer) getLocation(chargePointId string) (*entity.Location, *entity.ChargePoint) {
	if lb.database == nil {
		return nil, nil
	}
	chp, err := lb.database.GetChargePoint(chargePointId)
	if err != nil {
		lb.log.FeatureEvent(featureName, chargePointId, fmt.Sprintf("error getting charge point: %s", err))
		return nil, nil
	}
	if chp == nil {
		lb.log.FeatureEvent(featureName, chargePointId, "charge point not found in database")
		return nil, nil
	}
	if !chp.SmartCharging || chp.LocationId == "" {
		return nil, nil
	}
	location, err := lb.database.GetLocation(chp.LocationId)
	if err != nil {
		lb.log.FeatureEvent(featureName, chargePointId, fmt.Sprintf("error getting location: %s %s", chp.LocationId, err))
	}
	return location, chp
}

// updateConnectorPower sets or clears the power limit for a connector
// Handles both OCPP 1.6J connectors and OCPP 2.0.1 EVSEs
func (lb *LoadBalancer) updateConnectorPower(powerLimit int, connector *entity.Connector) error {
	if connector.CurrentTransactionId < 0 && connector.CurrentPowerLimit == 0 {
		// no need to update - connector is not active and has no limit set
		return nil
	}
	chargePointId := connector.ChargePointId
	if connector.CurrentTransactionId >= 0 {
		// Log with EVSE information if available (OCPP 2.0.1)
		connectorInfo := fmt.Sprintf("connector %d", connector.Id)
		if connector.EvseId != nil {
			connectorInfo = fmt.Sprintf("EVSE %d / connector %d", *connector.EvseId, connector.Id)
		}
		if err := lb.sendTransactionProfile(connector, powerLimit, connectorInfo); err != nil {
			return fmt.Errorf("sending profile update request: %s", err)
		}
		connector.CurrentPowerLimit = powerLimit
		if lb.database != nil {
			if err := lb.database.UpdateTransactionPowerLimit(connector.CurrentTransactionId, powerLimit); err != nil {
				lb.log.FeatureEvent(featureName, chargePointId, fmt.Sprintf("error updating transaction power limit: %s", err))
			}
		}
	} else {
		connectorInfo := fmt.Sprintf("connector %d", connector.Id)
		if connector.EvseId != nil {
			connectorInfo = fmt.Sprintf("EVSE %d / connector %d", *connector.EvseId, connector.Id)
		}
		lb.log.FeatureEvent(featureName, chargePointId, fmt.Sprintf("cleared power limit for %s", connectorInfo))
		connector.CurrentPowerLimit = 0
	}
	if lb.database != nil {
		err := lb.database.UpdateConnectorCurrentPower(connector)
		if err != nil {
			lb.log.FeatureEvent(featureName, chargePointId, fmt.Sprintf("database error: %s", err))
		}
	}
	return nil
}
