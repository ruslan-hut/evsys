package core

import (
	"evsys/types"
	"fmt"
	"reflect"
)

const BootNotificationFeatureName = "BootNotification"

// RegistrationStatus Result of registration in response to a BootNotification request.
type RegistrationStatus string

const (
	RegistrationStatusAccepted RegistrationStatus = "Accepted"
	RegistrationStatusPending  RegistrationStatus = "Pending"
	RegistrationStatusRejected RegistrationStatus = "Rejected"
)

type BootNotificationRequest struct {
	ChargePointVendor       string `json:"chargePointVendor"`
	ChargePointModel        string `json:"chargePointModel"`
	ChargePointSerialNumber string `json:"chargePointSerialNumber"`
	ChargeBoxSerialNumber   string `json:"chargeBoxSerialNumber"`
	FirmwareVersion         string `json:"firmwareVersion"`
	Iccid                   string `json:"iccid"`
	Imsi                    string `json:"imsi"`
	MeterType               string `json:"meterType"`
	MeterSerialNumber       string `json:"meterSerialNumber"`
}

type BootNotificationResponse struct {
	CurrentTime *types.DateTime    `json:"currentTime"`
	Interval    int                `json:"interval"`
	Status      RegistrationStatus `json:"status"`
}

// NewBootNotificationResponse Creates a new BootNotificationResponse. There are no optional fields for this message.
func NewBootNotificationResponse(currentTime *types.DateTime, interval int, status RegistrationStatus) *BootNotificationResponse {
	return &BootNotificationResponse{CurrentTime: currentTime, Interval: interval, Status: status}
}

func (r *BootNotificationRequest) GetFeatureName() string {
	return BootNotificationFeatureName
}

// Validate checks the fields mandated by the OCPP 1.6 spec.
func (r *BootNotificationRequest) Validate() error {
	if r.ChargePointVendor == "" {
		return fmt.Errorf("chargePointVendor is required")
	}
	if len(r.ChargePointVendor) > 20 {
		return fmt.Errorf("chargePointVendor exceeds 20 characters")
	}
	if r.ChargePointModel == "" {
		return fmt.Errorf("chargePointModel is required")
	}
	if len(r.ChargePointModel) > 20 {
		return fmt.Errorf("chargePointModel exceeds 20 characters")
	}
	return nil
}

func (r *BootNotificationResponse) GetFeatureName() string {
	return BootNotificationFeatureName
}

func (r *BootNotificationRequest) GetRequestType() reflect.Type {
	return reflect.TypeOf(BootNotificationRequest{})
}
