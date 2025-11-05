package v201

import (
	"evsys/ocpp/common"
	"time"
)

// ============================================================================
// OCPP 2.0.1 Base Type System
// ============================================================================
// This file defines the core types and enumerations used throughout OCPP 2.0.1
// Based on: OCPP 2.0.1 Specification - Part 2: Specification
// ============================================================================

// ============================================================================
// ENUMERATIONS
// ============================================================================

// IdTokenType defines the type of identification token
type IdTokenType string

const (
	IdTokenTypeCentral         IdTokenType = "Central"         // Token authorized by central system
	IdTokenTypeEMAID           IdTokenType = "eMAID"           // Electric Mobility Account ID (ISO 15118)
	IdTokenTypeISO14443        IdTokenType = "ISO14443"        // RFID card (ISO 14443)
	IdTokenTypeISO15693        IdTokenType = "ISO15693"        // RFID card (ISO 15693)
	IdTokenTypeKeyCode         IdTokenType = "KeyCode"         // User entered key code
	IdTokenTypeLocal           IdTokenType = "Local"           // Token authorized locally
	IdTokenTypeMacAddress      IdTokenType = "MacAddress"      // MAC address
	IdTokenTypeNoAuthorization IdTokenType = "NoAuthorization" // No authorization required
)

// AuthorizationStatusType defines the result of an authorization request
type AuthorizationStatusType string

const (
	AuthorizationStatusAccepted           AuthorizationStatusType = "Accepted"           // Token accepted
	AuthorizationStatusBlocked            AuthorizationStatusType = "Blocked"            // Token blocked
	AuthorizationStatusConcurrentTx       AuthorizationStatusType = "ConcurrentTx"       // Token already in use
	AuthorizationStatusExpired            AuthorizationStatusType = "Expired"            // Token expired
	AuthorizationStatusInvalid            AuthorizationStatusType = "Invalid"            // Token invalid
	AuthorizationStatusNoCredit           AuthorizationStatusType = "NoCredit"           // Insufficient credit
	AuthorizationStatusNotAllowedTypeEVSE AuthorizationStatusType = "NotAllowedTypeEVSE" // Not allowed at this EVSE type
	AuthorizationStatusNotAtThisLocation  AuthorizationStatusType = "NotAtThisLocation"  // Not allowed at this location
	AuthorizationStatusNotAtThisTime      AuthorizationStatusType = "NotAtThisTime"      // Not allowed at this time
	AuthorizationStatusUnknown            AuthorizationStatusType = "Unknown"            // Token unknown
)

// ConnectorStatusType defines the status of a connector
type ConnectorStatusType string

const (
	ConnectorStatusAvailable   ConnectorStatusType = "Available"   // Connector available for new transaction
	ConnectorStatusOccupied    ConnectorStatusType = "Occupied"    // Connector occupied (charging or about to charge)
	ConnectorStatusReserved    ConnectorStatusType = "Reserved"    // Connector reserved for specific ID token
	ConnectorStatusUnavailable ConnectorStatusType = "Unavailable" // Connector not available
	ConnectorStatusFaulted     ConnectorStatusType = "Faulted"     // Connector in faulted state
)

// ChargingStateType defines the charging state
type ChargingStateType string

const (
	ChargingStateCharging      ChargingStateType = "Charging"      // Charging in progress
	ChargingStateEVConnected   ChargingStateType = "EVConnected"   // EV connected, not charging
	ChargingStateSuspendedEV   ChargingStateType = "SuspendedEV"   // Suspended by EV
	ChargingStateSuspendedEVSE ChargingStateType = "SuspendedEVSE" // Suspended by EVSE
	ChargingStateIdle          ChargingStateType = "Idle"          // No EV connected
)

// TransactionEventType defines the type of transaction event
type TransactionEventType string

const (
	TransactionEventStarted TransactionEventType = "Started" // Transaction started
	TransactionEventUpdated TransactionEventType = "Updated" // Transaction updated
	TransactionEventEnded   TransactionEventType = "Ended"   // Transaction ended
)

// TriggerReasonType defines why a transaction event was triggered
type TriggerReasonType string

const (
	TriggerReasonAuthorized           TriggerReasonType = "Authorized"           // Authorized by ID token
	TriggerReasonCablePluggedIn       TriggerReasonType = "CablePluggedIn"       // Cable plugged in
	TriggerReasonChargingRateChanged  TriggerReasonType = "ChargingRateChanged"  // Charging rate changed
	TriggerReasonChargingStateChanged TriggerReasonType = "ChargingStateChanged" // Charging state changed
	TriggerReasonDeauthorized         TriggerReasonType = "Deauthorized"         // Deauthorized
	TriggerReasonEnergyLimitReached   TriggerReasonType = "EnergyLimitReached"   // Energy limit reached
	TriggerReasonEVCommunicationLost  TriggerReasonType = "EVCommunicationLost"  // Communication with EV lost
	TriggerReasonEVConnectTimeout     TriggerReasonType = "EVConnectTimeout"     // EV connection timeout
	TriggerReasonMeterValueClock      TriggerReasonType = "MeterValueClock"      // Regular meter value clock
	TriggerReasonMeterValuePeriodic   TriggerReasonType = "MeterValuePeriodic"   // Periodic meter value
	TriggerReasonTimeLimitReached     TriggerReasonType = "TimeLimitReached"     // Time limit reached
	TriggerReasonTrigger              TriggerReasonType = "Trigger"              // Triggered by TriggerMessage
	TriggerReasonUnlockCommand        TriggerReasonType = "UnlockCommand"        // Unlock command received
	TriggerReasonStopAuthorized       TriggerReasonType = "StopAuthorized"       // Stop authorized
	TriggerReasonEVDeparted           TriggerReasonType = "EVDeparted"           // EV departed
	TriggerReasonEVDetected           TriggerReasonType = "EVDetected"           // EV detected
	TriggerReasonRemoteStop           TriggerReasonType = "RemoteStop"           // Remote stop
	TriggerReasonRemoteStart          TriggerReasonType = "RemoteStart"          // Remote start
	TriggerReasonAbnormalCondition    TriggerReasonType = "AbnormalCondition"    // Abnormal condition
	TriggerReasonSignedDataReceived   TriggerReasonType = "SignedDataReceived"   // Signed data received
	TriggerReasonResetCommand         TriggerReasonType = "ResetCommand"         // Reset command
)

// ReasonType defines the reason for stopping a transaction
type ReasonType string

const (
	ReasonDeAuthorized     ReasonType = "DeAuthorized"     // De-authorized by central system
	ReasonEmergencyStop    ReasonType = "EmergencyStop"    // Emergency stop button pressed
	ReasonEVDisconnected   ReasonType = "EVDisconnected"   // EV disconnected
	ReasonGroundFault      ReasonType = "GroundFault"      // Ground fault detected
	ReasonImmediateReset   ReasonType = "ImmediateReset"   // Immediate reset command
	ReasonLocal            ReasonType = "Local"            // Stopped locally
	ReasonLocalOutOfCredit ReasonType = "LocalOutOfCredit" // Out of credit
	ReasonMasterPass       ReasonType = "MasterPass"       // Stopped by master pass
	ReasonOther            ReasonType = "Other"            // Other reason
	ReasonOvercurrentFault ReasonType = "OvercurrentFault" // Overcurrent fault
	ReasonPowerLoss        ReasonType = "PowerLoss"        // Power loss
	ReasonPowerQuality     ReasonType = "PowerQuality"     // Power quality issue
	ReasonReboot           ReasonType = "Reboot"           // System reboot
	ReasonRemote           ReasonType = "Remote"           // Stopped remotely
	ReasonSOCLimitReached  ReasonType = "SOCLimitReached"  // State of charge limit reached
	ReasonStoppedByEV      ReasonType = "StoppedByEV"      // Stopped by EV
	ReasonTimeLimitReached ReasonType = "TimeLimitReached" // Time limit reached
	ReasonTimeout          ReasonType = "Timeout"          // Timeout
)

// BootReasonType defines the reason for a boot notification
type BootReasonType string

const (
	BootReasonApplicationReset BootReasonType = "ApplicationReset" // Application reset
	BootReasonFirmwareUpdate   BootReasonType = "FirmwareUpdate"   // Firmware update
	BootReasonLocalReset       BootReasonType = "LocalReset"       // Local reset
	BootReasonPowerUp          BootReasonType = "PowerUp"          // Power up
	BootReasonRemoteReset      BootReasonType = "RemoteReset"      // Remote reset
	BootReasonScheduledReset   BootReasonType = "ScheduledReset"   // Scheduled reset
	BootReasonTriggered        BootReasonType = "Triggered"        // Triggered by remote request
	BootReasonUnknown          BootReasonType = "Unknown"          // Unknown reason
	BootReasonWatchdog         BootReasonType = "Watchdog"         // Watchdog reset
)

// RegistrationStatusType defines the result of a boot notification
type RegistrationStatusType string

const (
	RegistrationStatusAccepted RegistrationStatusType = "Accepted" // Accepted
	RegistrationStatusPending  RegistrationStatusType = "Pending"  // Pending
	RegistrationStatusRejected RegistrationStatusType = "Rejected" // Rejected
)

// MessagePriorityType defines message priority
type MessagePriorityType string

const (
	MessagePriorityAlwaysFront MessagePriorityType = "AlwaysFront" // Always front
	MessagePriorityInFront     MessagePriorityType = "InFront"     // In front
	MessagePriorityNormalCycle MessagePriorityType = "NormalCycle" // Normal cycle
)

// MessageStateType defines the state of a display message
type MessageStateType string

const (
	MessageStateCharging    MessageStateType = "Charging"    // During charging
	MessageStateFaulted     MessageStateType = "Faulted"     // When faulted
	MessageStateIdle        MessageStateType = "Idle"        // When idle
	MessageStateUnavailable MessageStateType = "Unavailable" // When unavailable
)

// ReadingContextType defines the context of a meter reading
type ReadingContextType string

const (
	ReadingContextInterruptionBegin ReadingContextType = "Interruption.Begin" // Begin of interruption
	ReadingContextInterruptionEnd   ReadingContextType = "Interruption.End"   // End of interruption
	ReadingContextOther             ReadingContextType = "Other"              // Other
	ReadingContextSampleClock       ReadingContextType = "Sample.Clock"       // Clock-based sample
	ReadingContextSamplePeriodic    ReadingContextType = "Sample.Periodic"    // Periodic sample
	ReadingContextTransactionBegin  ReadingContextType = "Transaction.Begin"  // Begin of transaction
	ReadingContextTransactionEnd    ReadingContextType = "Transaction.End"    // End of transaction
	ReadingContextTrigger           ReadingContextType = "Trigger"            // Triggered
)

// MeasurandType defines what is being measured
type MeasurandType string

const (
	MeasurandCurrentExport                MeasurandType = "Current.Export"                  // Current export
	MeasurandCurrentImport                MeasurandType = "Current.Import"                  // Current import
	MeasurandCurrentOffered               MeasurandType = "Current.Offered"                 // Current offered
	MeasurandEnergyActiveExportRegister   MeasurandType = "Energy.Active.Export.Register"   // Energy active export register
	MeasurandEnergyActiveImportRegister   MeasurandType = "Energy.Active.Import.Register"   // Energy active import register
	MeasurandEnergyReactiveExportRegister MeasurandType = "Energy.Reactive.Export.Register" // Energy reactive export
	MeasurandEnergyReactiveImportRegister MeasurandType = "Energy.Reactive.Import.Register" // Energy reactive import
	MeasurandEnergyActiveExportInterval   MeasurandType = "Energy.Active.Export.Interval"   // Energy active export interval
	MeasurandEnergyActiveImportInterval   MeasurandType = "Energy.Active.Import.Interval"   // Energy active import interval
	MeasurandEnergyActiveNet              MeasurandType = "Energy.Active.Net"               // Energy active net
	MeasurandEnergyReactiveExportInterval MeasurandType = "Energy.Reactive.Export.Interval" // Energy reactive export interval
	MeasurandEnergyReactiveImportInterval MeasurandType = "Energy.Reactive.Import.Interval" // Energy reactive import interval
	MeasurandEnergyReactiveNet            MeasurandType = "Energy.Reactive.Net"             // Energy reactive net
	MeasurandEnergyApparentNet            MeasurandType = "Energy.Apparent.Net"             // Energy apparent net
	MeasurandEnergyApparentImport         MeasurandType = "Energy.Apparent.Import"          // Energy apparent import
	MeasurandEnergyApparentExport         MeasurandType = "Energy.Apparent.Export"          // Energy apparent export
	MeasurandFrequency                    MeasurandType = "Frequency"                       // Frequency
	MeasurandPowerActiveExport            MeasurandType = "Power.Active.Export"             // Power active export
	MeasurandPowerActiveImport            MeasurandType = "Power.Active.Import"             // Power active import
	MeasurandPowerFactor                  MeasurandType = "Power.Factor"                    // Power factor
	MeasurandPowerOffered                 MeasurandType = "Power.Offered"                   // Power offered
	MeasurandPowerReactiveExport          MeasurandType = "Power.Reactive.Export"           // Power reactive export
	MeasurandPowerReactiveImport          MeasurandType = "Power.Reactive.Import"           // Power reactive import
	MeasurandSoC                          MeasurandType = "SoC"                             // State of charge
	MeasurandVoltage                      MeasurandType = "Voltage"                         // Voltage
)

// PhaseType defines the phase as used in SampledValue
type PhaseType string

const (
	PhaseL1   PhaseType = "L1"    // Phase 1
	PhaseL2   PhaseType = "L2"    // Phase 2
	PhaseL3   PhaseType = "L3"    // Phase 3
	PhaseN    PhaseType = "N"     // Neutral
	PhaseL1N  PhaseType = "L1-N"  // Phase 1 to neutral
	PhaseL2N  PhaseType = "L2-N"  // Phase 2 to neutral
	PhaseL3N  PhaseType = "L3-N"  // Phase 3 to neutral
	PhaseL1L2 PhaseType = "L1-L2" // Phase 1 to phase 2
	PhaseL2L3 PhaseType = "L2-L3" // Phase 2 to phase 3
	PhaseL3L1 PhaseType = "L3-L1" // Phase 3 to phase 1
)

// LocationType defines the location of a measurement
type LocationType string

const (
	LocationBody   LocationType = "Body"   // Body
	LocationCable  LocationType = "Cable"  // Cable
	LocationEV     LocationType = "EV"     // Electric vehicle
	LocationInlet  LocationType = "Inlet"  // Inlet
	LocationOutlet LocationType = "Outlet" // Outlet
)

// UnitOfMeasureType defines the unit of measure
type UnitOfMeasureType string

const (
	UnitWh         UnitOfMeasureType = "Wh"         // Watt-hour
	UnitKWh        UnitOfMeasureType = "kWh"        // Kilowatt-hour
	UnitVarh       UnitOfMeasureType = "varh"       // Var-hour
	UnitKvarh      UnitOfMeasureType = "kvarh"      // Kilovar-hour
	UnitW          UnitOfMeasureType = "W"          // Watt
	UnitKW         UnitOfMeasureType = "kW"         // Kilowatt
	UnitVA         UnitOfMeasureType = "VA"         // Volt-ampere
	UnitKVA        UnitOfMeasureType = "kVA"        // Kilovolt-ampere
	UnitVar        UnitOfMeasureType = "var"        // Var
	UnitKvar       UnitOfMeasureType = "kvar"       // Kilovar
	UnitA          UnitOfMeasureType = "A"          // Ampere
	UnitV          UnitOfMeasureType = "V"          // Volt
	UnitCelsius    UnitOfMeasureType = "Celsius"    // Celsius
	UnitFahrenheit UnitOfMeasureType = "Fahrenheit" // Fahrenheit
	UnitK          UnitOfMeasureType = "K"          // Kelvin
	UnitPercent    UnitOfMeasureType = "Percent"    // Percent
	UnitHz         UnitOfMeasureType = "Hz"         // Hertz
)

// ChargingProfilePurposeType defines the purpose of a charging profile
type ChargingProfilePurposeType string

const (
	ChargingProfilePurposeChargingStationMaxProfile ChargingProfilePurposeType = "ChargingStationMaxProfile" // Charging station max profile
	ChargingProfilePurposeTxDefaultProfile          ChargingProfilePurposeType = "TxDefaultProfile"          // Transaction default profile
	ChargingProfilePurposeTxProfile                 ChargingProfilePurposeType = "TxProfile"                 // Transaction profile
)

// ChargingProfileKindType defines the kind of charging profile
type ChargingProfileKindType string

const (
	ChargingProfileKindAbsolute  ChargingProfileKindType = "Absolute"  // Absolute
	ChargingProfileKindRecurring ChargingProfileKindType = "Recurring" // Recurring
	ChargingProfileKindRelative  ChargingProfileKindType = "Relative"  // Relative
)

// RecurrencyKindType defines the recurrency of a charging profile
type RecurrencyKindType string

const (
	RecurrencyKindDaily  RecurrencyKindType = "Daily"  // Daily
	RecurrencyKindWeekly RecurrencyKindType = "Weekly" // Weekly
)

// ChargingRateUnitType defines the unit of a charging rate
type ChargingRateUnitType string

const (
	ChargingRateUnitA ChargingRateUnitType = "A" // Amperes
	ChargingRateUnitW ChargingRateUnitType = "W" // Watts
)

// ============================================================================
// CORE DATA TYPES
// ============================================================================

// IdToken represents an authorization token in OCPP 2.0.1
// Replaces the IdTag string from OCPP 1.6J
type IdToken struct {
	// IdToken is the identifier (max 36 characters)
	IdToken string `json:"idToken" validate:"required,max=36"`

	// Type is the type of the token
	Type IdTokenType `json:"type" validate:"required"`

	// AdditionalInfo contains additional information about the token
	AdditionalInfo []AdditionalInfo `json:"additionalInfo,omitempty" validate:"omitempty,max=3,dive"`
}

// AdditionalInfo contains additional information about an IdToken
type AdditionalInfo struct {
	// AdditionalIdToken is an additional identifier
	AdditionalIdToken string `json:"additionalIdToken" validate:"required,max=36"`

	// Type is the type of the additional identifier
	Type string `json:"type" validate:"required,max=50"`
}

// IdTokenInfo contains authorization information about an IdToken
type IdTokenInfo struct {
	// Status is the authorization status
	Status AuthorizationStatusType `json:"status" validate:"required"`

	// CacheExpiryDateTime is when the authorization cache expires
	CacheExpiryDateTime *time.Time `json:"cacheExpiryDateTime,omitempty"`

	// ChargingPriority is the priority for charging (range -9 to 9)
	ChargingPriority *int `json:"chargingPriority,omitempty" validate:"omitempty,min=-9,max=9"`

	// Language1 is the preferred UI language (RFC 5646)
	Language1 string `json:"language1,omitempty" validate:"omitempty,max=8"`

	// Language2 is the second preferred UI language
	Language2 string `json:"language2,omitempty" validate:"omitempty,max=8"`

	// GroupIdToken is the group identifier
	GroupIdToken *IdToken `json:"groupIdToken,omitempty"`

	// PersonalMessage is a message to be displayed to the user
	PersonalMessage *MessageContent `json:"personalMessage,omitempty"`
}

// MessageContent represents a message to be displayed
type MessageContent struct {
	// Content is the message content
	Content string `json:"content" validate:"required,max=512"`

	// Format is the format of the message (e.g., "UTF8", "ASCII")
	Format string `json:"format" validate:"required,max=20"`

	// Language is the language of the message (RFC 5646)
	Language string `json:"language,omitempty" validate:"omitempty,max=8"`
}

// EVSE represents an Electric Vehicle Supply Equipment
// Hierarchical structure: ChargingStation → EVSE → Connector
type EVSE struct {
	// Id is the EVSE identifier (min 1)
	Id int `json:"id" validate:"required,min=1"`

	// ConnectorId is the connector identifier (optional in some contexts)
	ConnectorId *int `json:"connectorId,omitempty" validate:"omitempty,min=1"`
}

// Connector represents a physical connector on an EVSE
type Connector struct {
	// ConnectorId is the connector identifier
	ConnectorId int `json:"connectorId" validate:"required,min=1"`

	// ConnectorType is the type of connector (e.g., "cType2", "cCCS1", "cCCS2")
	ConnectorType string `json:"connectorType,omitempty" validate:"omitempty,max=20"`
}

// StatusInfo provides additional status information
type StatusInfo struct {
	// ReasonCode is a predefined code for the status
	ReasonCode string `json:"reasonCode" validate:"required,max=20"`

	// AdditionalInfo is additional information about the status
	AdditionalInfo string `json:"additionalInfo,omitempty" validate:"omitempty,max=512"`
}

// ChargingStation represents the charging station information
type ChargingStation struct {
	// Model is the charging station model
	Model string `json:"model" validate:"required,max=20"`

	// VendorName is the charging station vendor name
	VendorName string `json:"vendorName" validate:"required,max=50"`

	// SerialNumber is the charging station serial number
	SerialNumber string `json:"serialNumber,omitempty" validate:"omitempty,max=25"`

	// FirmwareVersion is the firmware version
	FirmwareVersion string `json:"firmwareVersion,omitempty" validate:"omitempty,max=50"`

	// Modem contains modem information
	Modem *Modem `json:"modem,omitempty"`
}

// Modem contains information about the modem
type Modem struct {
	// Iccid is the integrated circuit card identifier
	Iccid string `json:"iccid,omitempty" validate:"omitempty,max=20"`

	// Imsi is the international mobile subscriber identity
	Imsi string `json:"imsi,omitempty" validate:"omitempty,max=20"`
}

// Transaction represents a transaction in OCPP 2.0.1
type Transaction struct {
	// TransactionId is the transaction identifier
	TransactionId string `json:"transactionId" validate:"required,max=36"`

	// ChargingState is the current charging state
	ChargingState ChargingStateType `json:"chargingState,omitempty"`

	// TimeSpentCharging is the total time spent charging (seconds)
	TimeSpentCharging *int `json:"timeSpentCharging,omitempty" validate:"omitempty,min=0"`

	// StoppedReason is the reason for stopping
	StoppedReason ReasonType `json:"stoppedReason,omitempty"`

	// RemoteStartId is the ID given in RequestStartTransaction
	RemoteStartId *int `json:"remoteStartId,omitempty"`
}

// SampledValue represents a single sampled meter value
type SampledValue struct {
	// Value is the measured value
	Value float64 `json:"value" validate:"required"`

	// Context is the reading context
	Context ReadingContextType `json:"context,omitempty"`

	// Measurand is what is being measured
	Measurand MeasurandType `json:"measurand,omitempty"`

	// Phase is the phase for which the value is measured
	Phase PhaseType `json:"phase,omitempty"`

	// Location is the location where the value is measured
	Location LocationType `json:"location,omitempty"`

	// SignedMeterValue contains signed meter data
	SignedMeterValue *SignedMeterValue `json:"signedMeterValue,omitempty"`

	// UnitOfMeasure is the unit of measure
	UnitOfMeasure *UnitOfMeasure `json:"unitOfMeasure,omitempty"`
}

// SignedMeterValue contains signed meter data
type SignedMeterValue struct {
	// SignedMeterData is the signed meter data
	SignedMeterData string `json:"signedMeterData" validate:"required,max=2500"`

	// SigningMethod is the method used for signing
	SigningMethod string `json:"signingMethod" validate:"required,max=50"`

	// EncodingMethod is the encoding method used
	EncodingMethod string `json:"encodingMethod" validate:"required,max=50"`

	// PublicKey is the public key used for verification
	PublicKey string `json:"publicKey" validate:"required,max=2500"`
}

// UnitOfMeasure defines the unit of measure for a sampled value
type UnitOfMeasure struct {
	// Unit is the unit of measure
	Unit UnitOfMeasureType `json:"unit,omitempty"`

	// Multiplier is the multiplier for the unit (default 0 = *1)
	Multiplier *int `json:"multiplier,omitempty" validate:"omitempty,min=-3,max=3"`
}

// MeterValue represents a collection of meter values
type MeterValue struct {
	// Timestamp is when the meter value was sampled
	Timestamp time.Time `json:"timestamp" validate:"required"`

	// SampledValue is the array of sampled values
	SampledValue []SampledValue `json:"sampledValue" validate:"required,min=1,dive"`
}

// Component represents a component in the device model
type Component struct {
	// Name is the component name
	Name string `json:"name" validate:"required,max=50"`

	// Instance is the component instance
	Instance string `json:"instance,omitempty" validate:"omitempty,max=50"`

	// Evse is the EVSE this component belongs to
	Evse *EVSE `json:"evse,omitempty"`
}

// Variable represents a variable in the device model
type Variable struct {
	// Name is the variable name
	Name string `json:"name" validate:"required,max=50"`

	// Instance is the variable instance
	Instance string `json:"instance,omitempty" validate:"omitempty,max=50"`
}

// ChargingProfile defines a charging profile
type ChargingProfile struct {
	// Id is the unique identifier for this profile
	Id int `json:"id" validate:"required"`

	// StackLevel defines the level of this profile (0-based)
	StackLevel int `json:"stackLevel" validate:"required,min=0"`

	// ChargingProfilePurpose defines the purpose
	ChargingProfilePurpose ChargingProfilePurposeType `json:"chargingProfilePurpose" validate:"required"`

	// ChargingProfileKind defines the kind
	ChargingProfileKind ChargingProfileKindType `json:"chargingProfileKind" validate:"required"`

	// RecurrencyKind defines the recurrency (required if kind is Recurring)
	RecurrencyKind RecurrencyKindType `json:"recurrencyKind,omitempty"`

	// ValidFrom is when the profile becomes valid
	ValidFrom *time.Time `json:"validFrom,omitempty"`

	// ValidTo is when the profile becomes invalid
	ValidTo *time.Time `json:"validTo,omitempty"`

	// TransactionId associates this profile with a transaction
	TransactionId string `json:"transactionId,omitempty" validate:"omitempty,max=36"`

	// ChargingSchedule defines the charging schedule
	ChargingSchedule []ChargingSchedule `json:"chargingSchedule" validate:"required,min=1,max=3,dive"`
}

// ChargingSchedule defines a charging schedule
type ChargingSchedule struct {
	// Id is the unique identifier for this schedule
	Id int `json:"id" validate:"required"`

	// StartSchedule is when the schedule starts (optional for relative schedules)
	StartSchedule *time.Time `json:"startSchedule,omitempty"`

	// Duration is the duration of the schedule in seconds
	Duration *int `json:"duration,omitempty" validate:"omitempty,min=0"`

	// ChargingRateUnit is the unit for the charging rate
	ChargingRateUnit ChargingRateUnitType `json:"chargingRateUnit" validate:"required"`

	// ChargingSchedulePeriod defines the schedule periods
	ChargingSchedulePeriod []ChargingSchedulePeriod `json:"chargingSchedulePeriod" validate:"required,min=1,dive"`

	// MinChargingRate is the minimum charging rate
	MinChargingRate *float64 `json:"minChargingRate,omitempty" validate:"omitempty,min=0"`

	// SalesTariff is the sales tariff associated with this schedule
	SalesTariff *SalesTariff `json:"salesTariff,omitempty"`
}

// ChargingSchedulePeriod defines a time period in a charging schedule
type ChargingSchedulePeriod struct {
	// StartPeriod is the start of the period in seconds from schedule start
	StartPeriod int `json:"startPeriod" validate:"required,min=0"`

	// Limit is the charging rate limit
	Limit float64 `json:"limit" validate:"required"`

	// NumberPhases is the number of phases to use
	NumberPhases *int `json:"numberPhases,omitempty" validate:"omitempty,min=1,max=3"`

	// PhaseToUse is which phase to use for single-phase charging
	PhaseToUse *int `json:"phaseToUse,omitempty" validate:"omitempty,min=1,max=3"`
}

// SalesTariff defines a sales tariff
type SalesTariff struct {
	// Id is the tariff identifier
	Id int `json:"id" validate:"required"`

	// SalesTariffDescription is the description of the tariff
	SalesTariffDescription string `json:"salesTariffDescription,omitempty" validate:"omitempty,max=32"`

	// NumEPriceLevels is the number of price levels
	NumEPriceLevels *int `json:"numEPriceLevels,omitempty" validate:"omitempty,min=1"`

	// SalesTariffEntry defines the tariff entries
	SalesTariffEntry []SalesTariffEntry `json:"salesTariffEntry" validate:"required,min=1,dive"`
}

// SalesTariffEntry defines a single entry in a sales tariff
type SalesTariffEntry struct {
	// RelativeTimeInterval defines the time interval
	RelativeTimeInterval RelativeTimeInterval `json:"relativeTimeInterval" validate:"required"`

	// EPriceLevel is the price level (if multiple levels are defined)
	EPriceLevel *int `json:"ePriceLevel,omitempty" validate:"omitempty,min=0"`

	// ConsumptionCost defines the costs
	ConsumptionCost []ConsumptionCost `json:"consumptionCost,omitempty" validate:"omitempty,max=3,dive"`
}

// RelativeTimeInterval defines a time interval relative to a reference
type RelativeTimeInterval struct {
	// Start is the start of the interval in seconds
	Start int `json:"start" validate:"required,min=0"`

	// Duration is the duration in seconds
	Duration *int `json:"duration,omitempty" validate:"omitempty,min=0"`
}

// ConsumptionCost defines a cost based on consumption
type ConsumptionCost struct {
	// StartValue is the start value for this cost
	StartValue float64 `json:"startValue" validate:"required"`

	// Cost defines the cost
	Cost []Cost `json:"cost" validate:"required,min=1,max=3,dive"`
}

// Cost defines a cost item
type Cost struct {
	// CostKind defines the kind of cost
	CostKind string `json:"costKind" validate:"required"`

	// Amount is the cost amount
	Amount int `json:"amount" validate:"required"`

	// AmountMultiplier is the multiplier for the amount
	AmountMultiplier *int `json:"amountMultiplier,omitempty" validate:"omitempty,min=-3,max=3"`
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// GetFeatureName implements common.Request interface for all request types
// Each specific request type should implement this method to return its feature name
func (r *IdToken) GetProtocolVersion() common.ProtocolVersion {
	return common.OCPP201
}

// Validate validates the IdToken
func (t *IdToken) Validate() error {
	if t.IdToken == "" || len(t.IdToken) > 36 {
		return ErrInvalidIdToken
	}
	if t.Type == "" {
		return ErrInvalidIdTokenType
	}
	if len(t.AdditionalInfo) > 3 {
		return ErrTooManyAdditionalInfo
	}
	return nil
}

// ============================================================================
// ERROR DEFINITIONS
// ============================================================================

// Common validation errors
var (
	ErrInvalidIdToken        = &ValidationError{Field: "idToken", Message: "invalid ID token"}
	ErrInvalidIdTokenType    = &ValidationError{Field: "type", Message: "invalid ID token type"}
	ErrTooManyAdditionalInfo = &ValidationError{Field: "additionalInfo", Message: "too many additional info items (max 3)"}
)

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
