package ocpp

import (
	"evsys/types"
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

func (r *BootNotificationResponse) GetFeatureName() string {
	return BootNotificationFeatureName
}

func (r *BootNotificationRequest) GetRequestType() reflect.Type {
	return reflect.TypeOf(BootNotificationRequest{})
}
