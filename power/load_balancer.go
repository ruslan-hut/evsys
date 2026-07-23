package power

import (
	"encoding/json"
	"evsys/entity"
	"evsys/internal"
	"evsys/ocpp"
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
)

// profileStatus is the shape shared by the SetChargingProfile and
// ClearChargingProfile responses. Both report "Accepted" on success.
type profileStatus struct {
	Status string `json:"status"`
}

const profileAccepted = "Accepted"

// powerSlots are assigned highest-first, one connector per slot;
// when all slots are taken, further connectors get baseLimit
var powerSlots = []int{330, 195, 145, 115}

type LoadBalancer struct {
	database Repository
	server   Handler
	log      internal.LogHandler
	mutex    sync.Mutex
}

func NewLoadBalancer(database Repository, server Handler, log internal.LogHandler) *LoadBalancer {
	return &LoadBalancer{
		database: database,
		server:   server,
		log:      log,
		mutex:    sync.Mutex{},
	}
}

func (lb *LoadBalancer) OnChargePointBoot(chargePointId string) {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()
	location, _ := lb.getLocation(chargePointId)
	if location == nil {
		return
	}
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
	if err := lb.sendProfile(chargePointId, description, request); err != nil {
		lb.log.FeatureEvent(featureName, chargePointId, fmt.Sprintf("error sending request: %s", err))
	}
}

// sendProfile queues a charging profile request and logs the charge point's
// verdict once it arrives. A charge point that rejects a profile keeps charging
// at whatever its own hardware allows, which looks identical to a limit
// correctly applied unless the response is read - so the verdict is recorded
// even though nothing acts on it yet.
func (lb *LoadBalancer) sendProfile(chargePointId, description string, request ocpp.Request) error {
	response, release, err := lb.server.SendRequestWithResponse(chargePointId, request)
	if err != nil {
		return err
	}
	go func() {
		defer release()
		select {
		case payload := <-response:
			var status profileStatus
			if err := json.Unmarshal([]byte(payload), &status); err != nil {
				lb.log.FeatureEvent(featureName, chargePointId,
					fmt.Sprintf("%s: unreadable response: %s", description, payload))
				return
			}
			if status.Status != profileAccepted {
				lb.log.FeatureEvent(featureName, chargePointId,
					fmt.Sprintf("%s: REJECTED by charge point, status %s", description, status.Status))
				return
			}
			lb.log.FeatureEvent(featureName, chargePointId, fmt.Sprintf("%s: accepted", description))
		case <-time.After(profileResponseTimeout):
			lb.log.FeatureEvent(featureName, chargePointId,
				fmt.Sprintf("%s: no response within %s", description, profileResponseTimeout))
		}
	}()
	return nil
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

func (lb *LoadBalancer) CheckPowerLimit(chargePointId string) {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()
	location, _ := lb.getLocation(chargePointId)
	if location == nil {
		return
	}
	if location.PowerLimit == 0 {
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
		if chp.SmartCharging {
			for _, connector := range chp.Connectors {
				if connector.CurrentTransactionId >= 0 && connector.CurrentPowerLimit > 0 {
					continue
				}
				err := lb.updateConnectorPower(powerLimit, connector)
				if err != nil {
					lb.log.FeatureEvent(featureName, chargePointId, fmt.Sprintf("error updating connector: %s", err))
				}
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
		description := fmt.Sprintf("power limit %dA for %s", powerLimit, connectorInfo)
		lb.log.FeatureEvent(featureName, chargePointId, fmt.Sprintf("setting power limit to %dA for %s", powerLimit, connectorInfo))

		request := smartcharging.NewSetChargingProfileRequest(
			connector.Id, smartcharging.NewTransactionChargingProfile(
				connector.Id,
				connector.CurrentTransactionId,
				powerLimit))
		if err := lb.sendProfile(chargePointId, description, request); err != nil {
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
