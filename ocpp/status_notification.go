package ocpp

import "evsys/types"

const StatusNotificationFeatureName = "StatusNotification"

type ChargePointErrorCode string

type ChargePointStatus string

const (
	ConnectorLockFailure           ChargePointErrorCode = "ConnectorLockFailure"
	EVCommunicationError           ChargePointErrorCode = "EVCommunicationError"
	GroundFailure                  ChargePointErrorCode = "GroundFailure"
	HighTemperature                ChargePointErrorCode = "HighTemperature"
	InternalError                  ChargePointErrorCode = "InternalError"
	LocalListConflict              ChargePointErrorCode = "LocalListConflict"
	NoError                        ChargePointErrorCode = "NoError"
	OtherError                     ChargePointErrorCode = "OtherError"
	OverCurrentFailure             ChargePointErrorCode = "OverCurrentFailure"
	OverVoltage                    ChargePointErrorCode = "OverVoltage"
	PowerMeterFailure              ChargePointErrorCode = "PowerMeterFailure"
	PowerSwitchFailure             ChargePointErrorCode = "PowerSwitchFailure"
	ReaderFailure                  ChargePointErrorCode = "ReaderFailure"
	ResetFailure                   ChargePointErrorCode = "ResetFailure"
	UnderVoltage                   ChargePointErrorCode = "UnderVoltage"
	WeakSignal                     ChargePointErrorCode = "WeakSignal"
	ChargePointStatusAvailable     ChargePointStatus    = "Available"
	ChargePointStatusPreparing     ChargePointStatus    = "Preparing"
	ChargePointStatusCharging      ChargePointStatus    = "Charging"
	ChargePointStatusSuspendedEVSE ChargePointStatus    = "SuspendedEVSE"
	ChargePointStatusSuspendedEV   ChargePointStatus    = "SuspendedEV"
	ChargePointStatusFinishing     ChargePointStatus    = "Finishing"
	ChargePointStatusReserved      ChargePointStatus    = "Reserved"
	ChargePointStatusUnavailable   ChargePointStatus    = "Unavailable"
	ChargePointStatusFaulted       ChargePointStatus    = "Faulted"
)

type StatusNotificationRequest struct {
	ConnectorId     int                  `json:"connectorId" validate:"gte=0"`
	ErrorCode       ChargePointErrorCode `json:"errorCode" validate:"required,chargePointErrorCode"`
	Info            string               `json:"info,omitempty" validate:"max=50"`
	Status          ChargePointStatus    `json:"status" validate:"required,chargePointStatus"`
	Timestamp       *types.DateTime      `json:"timestamp,omitempty" validate:"omitempty"`
	VendorId        string               `json:"vendorId,omitempty" validate:"max=255"`
	VendorErrorCode string               `json:"vendorErrorCode,omitempty" validate:"max=50"`
}

type StatusNotificationResponse struct {
}

func (r StatusNotificationRequest) GetFeatureName() string {
	return StatusNotificationFeatureName
}

func (c StatusNotificationResponse) GetFeatureName() string {
	return StatusNotificationFeatureName
}

func NewStatusNotificationResponse() *StatusNotificationResponse {
	return &StatusNotificationResponse{}
}
