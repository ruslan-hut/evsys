package power

import (
	"evsys/entity"
	"evsys/internal"
	"evsys/ocpp"
	"evsys/ocpp/smartcharging"
	"fmt"
	"sync"
)

const (
	featureName = "LoadBalancer"
	baseLimit   = 50
	highLimit   = 150
	midLimit    = 90
)

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
	if location.DefaultPowerLimit == 0 {
		lb.log.FeatureEvent(featureName, chargePointId, "clearing default charging profile")
		request = smartcharging.NewClearDefaultChargingProfileRequest()
	} else {
		lb.log.FeatureEvent(featureName, chargePointId, fmt.Sprintf("setting default charging profile to %dA", location.DefaultPowerLimit))
		request = smartcharging.NewSetChargingProfileRequest(0, smartcharging.NewDefaultChargingProfile(location.DefaultPowerLimit))
	}
	_, err := lb.server.SendRequest(chargePointId, request)
	if err != nil {
		lb.log.FeatureEvent(featureName, chargePointId, fmt.Sprintf("error sending request: %s", err))
	}
}

func (lb *LoadBalancer) OnSystemStart() {
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
	activeHigh := false
	activeMid := false
	// all active connectors on smart charging points
	activeConnectors := 0
	for _, chp := range location.Evses {
		if chp.SmartCharging {
			for _, connector := range chp.Connectors {
				if connector.CurrentTransactionId >= 0 {
					activeConnectors++
					if connector.CurrentPowerLimit == highLimit {
						activeHigh = true
					}
					if connector.CurrentPowerLimit == midLimit {
						activeMid = true
					}
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
	lb.log.FeatureEvent(featureName, chargePointId, fmt.Sprintf("active connectors: %d; high=%v; mid=%v", activeConnectors, activeHigh, activeMid))

	powerLimit := baseLimit
	if !activeHigh {
		powerLimit = highLimit
	} else if !activeMid {
		powerLimit = midLimit
	}

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

func (lb *LoadBalancer) updateConnectorPower(powerLimit int, connector *entity.Connector) error {
	if connector.CurrentTransactionId < 0 && connector.CurrentPowerLimit == 0 {
		// no need to update - connector is not active and has no limit set
		return nil
	}
	chargePointId := connector.ChargePointId
	if connector.CurrentTransactionId >= 0 {
		lb.log.FeatureEvent(featureName, chargePointId, fmt.Sprintf("setting power limit to %dA for connector %d", powerLimit, connector.Id))
		request := smartcharging.NewSetChargingProfileRequest(
			connector.Id, smartcharging.NewTransactionChargingProfile(
				connector.CurrentTransactionId,
				powerLimit))
		_, err := lb.server.SendRequest(chargePointId, request)
		if err != nil {
			return fmt.Errorf("sending profile update request: %s", err)
		}
		connector.CurrentPowerLimit = powerLimit
	} else {
		lb.log.FeatureEvent(featureName, chargePointId, fmt.Sprintf("cleared power limit for connector %d", connector.Id))
		connector.CurrentPowerLimit = 0
	}
	err := lb.database.UpdateConnectorCurrentPower(connector)
	if err != nil {
		lb.log.FeatureEvent(featureName, chargePointId, fmt.Sprintf("database error: %s", err))
	}
	return nil
}
