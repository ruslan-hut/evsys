package power

import (
	"evsys/internal"
	"evsys/models"
	"evsys/ocpp"
	"evsys/ocpp/smartcharging"
	"fmt"
	"sync"
)

const featureName = "LoadBalancer"

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

func (lb *LoadBalancer) OnChargePointBoot(chargePointId string) error {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()
	location, _ := lb.getLocation(chargePointId)
	if location == nil {
		return nil
	}
	var request ocpp.Request
	if location.DefaultPowerLimit == 0 {
		lb.log.FeatureEvent(featureName, chargePointId, "clearing default charging profile")
		request = smartcharging.NewClearDefaultChargingProfileRequest()
	} else {
		lb.log.FeatureEvent(featureName, chargePointId, fmt.Sprintf("setting default charging profile to %d", location.DefaultPowerLimit))
		request = smartcharging.NewSetChargingProfileRequest(0, smartcharging.NewDefaultChargingProfile(location.DefaultPowerLimit))
	}
	_, err := lb.server.SendRequest(chargePointId, request)
	if err != nil {
		lb.log.FeatureEvent(featureName, chargePointId, fmt.Sprintf("error sending request: %s", err))
	}
	return nil
}

func (lb *LoadBalancer) BeforeNewTransaction(string) error {
	return nil
}

func (lb *LoadBalancer) CheckPowerLimit(chargePointId string) error {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()
	location, _ := lb.getLocation(chargePointId)
	if location == nil {
		return nil
	}
	if location.PowerLimit == 0 {
		return nil
	}
	// all active connectors on smart charging points
	totalConnectors := 0
	for _, chp := range location.Evses {
		if chp.SmartCharging {
			for _, connector := range chp.Connectors {
				if connector.CurrentTransactionId >= 0 {
					totalConnectors++
				}
			}
		}
	}
	if totalConnectors == 0 {
		return nil
	}
	// calculate power limit per connector
	powerLimitPerConnector := 10 * ((location.PowerLimit / 10) / totalConnectors)
	// send set charging profile request to each active connector
	for _, chp := range location.Evses {
		if chp.SmartCharging {
			for _, connector := range chp.Connectors {
				if connector.CurrentTransactionId >= 0 {
					lb.log.FeatureEvent(featureName, chp.Id, fmt.Sprintf("setting power limit to %d for connector %d", powerLimitPerConnector, connector.Id))
					request := smartcharging.NewSetChargingProfileRequest(
						connector.Id, smartcharging.NewTransactionChargingProfile(
							connector.CurrentTransactionId,
							powerLimitPerConnector))
					_, err := lb.server.SendRequest(chp.Id, request)
					if err != nil {
						lb.log.FeatureEvent(featureName, chp.Id, fmt.Sprintf("error sending request: %s", err))
					}
					err = lb.database.UpdateConnectorCurrentPower(connector)
					if err != nil {
						lb.log.FeatureEvent(featureName, chp.Id, fmt.Sprintf("error updating connector power: %s", err))
					}
				}
			}
		}
	}
	return nil
}

func (lb *LoadBalancer) getLocation(chargePointId string) (*models.Location, *models.ChargePoint) {
	chp, err := lb.database.GetChargePoint(chargePointId)
	if err != nil {
		lb.log.FeatureEvent(featureName, chargePointId, fmt.Sprintf("error getting charge point: %s", err))
	}
	if chp == nil {
		lb.log.FeatureEvent(featureName, chargePointId, "charge point not found")
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
