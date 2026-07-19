package core

import (
	"evsys/types"
	"fmt"
	"time"
)

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

// IsValid reports whether the status is a known OCPP 1.6 charge point status.
func (s ChargePointStatus) IsValid() bool {
	switch s {
	case ChargePointStatusAvailable, ChargePointStatusPreparing, ChargePointStatusCharging,
		ChargePointStatusSuspendedEVSE, ChargePointStatusSuspendedEV, ChargePointStatusFinishing,
		ChargePointStatusReserved, ChargePointStatusUnavailable, ChargePointStatusFaulted:
		return true
	default:
		return false
	}
}

// IsValid reports whether the error code is a known OCPP 1.6 error code.
func (e ChargePointErrorCode) IsValid() bool {
	switch e {
	case ConnectorLockFailure, EVCommunicationError, GroundFailure, HighTemperature, InternalError,
		LocalListConflict, NoError, OtherError, OverCurrentFailure, OverVoltage, PowerMeterFailure,
		PowerSwitchFailure, ReaderFailure, ResetFailure, UnderVoltage, WeakSignal:
		return true
	default:
		return false
	}
}

// Validate checks required fields and enum validity. Timestamp is optional per
// the OCPP 1.6 spec and is therefore not required here.
func (r StatusNotificationRequest) Validate() error {
	if r.ConnectorId < 0 {
		return fmt.Errorf("connectorId must be >= 0")
	}
	if r.Status == "" || !r.Status.IsValid() {
		return fmt.Errorf("invalid status: %q", r.Status)
	}
	if r.ErrorCode == "" || !r.ErrorCode.IsValid() {
		return fmt.Errorf("invalid errorCode: %q", r.ErrorCode)
	}
	if len(r.Info) > 50 {
		return fmt.Errorf("info exceeds 50 characters")
	}
	if len(r.VendorId) > 255 {
		return fmt.Errorf("vendorId exceeds 255 characters")
	}
	return nil
}

func (c StatusNotificationResponse) GetFeatureName() string {
	return StatusNotificationFeatureName
}

func NewStatusNotificationResponse() *StatusNotificationResponse {
	return &StatusNotificationResponse{}
}

func GetStatus(status string) ChargePointStatus {
	switch status {
	case "Available":
		return ChargePointStatusAvailable
	case "Preparing":
		return ChargePointStatusPreparing
	case "Charging":
		return ChargePointStatusCharging
	case "SuspendedEVSE":
		return ChargePointStatusSuspendedEVSE
	case "SuspendedEV":
		return ChargePointStatusSuspendedEV
	case "Finishing":
		return ChargePointStatusFinishing
	case "Reserved":
		return ChargePointStatusReserved
	case "Unavailable":
		return ChargePointStatusUnavailable
	case "Faulted":
		return ChargePointStatusFaulted
	default:
		return ChargePointStatusAvailable
	}
}

func GetErrorCode(errorCode string) ChargePointErrorCode {
	switch errorCode {
	case "ConnectorLockFailure":
		return ConnectorLockFailure
	case "EVCommunicationError":
		return EVCommunicationError
	case "GroundFailure":
		return GroundFailure
	case "HighTemperature":
		return HighTemperature
	case "InternalError":
		return InternalError
	case "LocalListConflict":
		return LocalListConflict
	case "NoError":
		return NoError
	case "OtherError":
		return OtherError
	case "OverCurrentFailure":
		return OverCurrentFailure
	case "OverVoltage":
		return OverVoltage
	case "PowerMeterFailure":
		return PowerMeterFailure
	case "PowerSwitchFailure":
		return PowerSwitchFailure
	case "ReaderFailure":
		return ReaderFailure
	case "ResetFailure":
		return ResetFailure
	case "UnderVoltage":
		return UnderVoltage
	case "WeakSignal":
		return WeakSignal
	default:
		return NoError
	}
}

// GetTimestamp get time.Time from DateTime
func (r StatusNotificationRequest) GetTimestamp() time.Time {
	if r.Timestamp == nil {
		return time.Now()
	}
	return r.Timestamp.Time
}
