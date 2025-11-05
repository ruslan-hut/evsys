package provisioning

import (
	"evsys/ocpp/common"
	"evsys/ocpp/v201"
	"time"
)

// ============================================================================
// NotifyReport - OCPP 2.0.1
// ============================================================================
// Sent by: Charging Station → CSMS
// Purpose: The Charging Station sends a NotifyReportRequest to report
//          monitoring data to the CSMS.
// ============================================================================

const NotifyReportFeatureName = "NotifyReport"

// ReportBaseType defines the type of report base
type ReportBaseType string

const (
	ReportBaseConfigurationInventory ReportBaseType = "ConfigurationInventory" // Configuration inventory
	ReportBaseFullInventory          ReportBaseType = "FullInventory"          // Full inventory
	ReportBaseSummaryInventory       ReportBaseType = "SummaryInventory"       // Summary inventory
)

// AttributeType defines the type of attribute
type AttributeType string

const (
	AttributeTypeActual AttributeType = "Actual" // Actual value
	AttributeTypeTarget AttributeType = "Target" // Target value
	AttributeTypeMinSet AttributeType = "MinSet" // Minimum settable value
	AttributeTypeMaxSet AttributeType = "MaxSet" // Maximum settable value
)

// MutabilityType defines the mutability of a variable
type MutabilityType string

const (
	MutabilityReadOnly  MutabilityType = "ReadOnly"  // Read-only
	MutabilityWriteOnly MutabilityType = "WriteOnly" // Write-only
	MutabilityReadWrite MutabilityType = "ReadWrite" // Read-write
)

// DataType defines the data type of a variable
type DataType string

const (
	DataTypeString       DataType = "string"       // String
	DataTypeDecimal      DataType = "decimal"      // Decimal number
	DataTypeInteger      DataType = "integer"      // Integer
	DataTypeDateTime     DataType = "dateTime"     // Date and time
	DataTypeBoolean      DataType = "boolean"      // Boolean
	DataTypeOptionList   DataType = "OptionList"   // Option list
	DataTypeSequenceList DataType = "SequenceList" // Sequence list
	DataTypeMemberList   DataType = "MemberList"   // Member list
)

// VariableAttribute represents an attribute of a variable
type VariableAttribute struct {
	// Type is the type of attribute
	Type AttributeType `json:"type,omitempty"`

	// Value is the value of the attribute
	Value string `json:"value,omitempty" validate:"omitempty,max=2500"`

	// Mutability indicates if the attribute is read-only, write-only, or read-write
	Mutability MutabilityType `json:"mutability,omitempty"`

	// Persistent indicates if the value persists across reboots
	Persistent *bool `json:"persistent,omitempty"`

	// Constant indicates if the value is constant
	Constant *bool `json:"constant,omitempty"`
}

// VariableCharacteristics describes the characteristics of a variable
type VariableCharacteristics struct {
	// DataType is the data type of the variable
	DataType DataType `json:"dataType" validate:"required"`

	// SupportsMonitoring indicates if the variable supports monitoring
	SupportsMonitoring bool `json:"supportsMonitoring"`

	// Unit is the unit of measure
	Unit string `json:"unit,omitempty" validate:"omitempty,max=20"`

	// MinLimit is the minimum value
	MinLimit *float64 `json:"minLimit,omitempty"`

	// MaxLimit is the maximum value
	MaxLimit *float64 `json:"maxLimit,omitempty"`

	// ValuesList is a comma-separated list of allowed values
	ValuesList string `json:"valuesList,omitempty" validate:"omitempty,max=1000"`
}

// ReportData contains information about a variable in a report
type ReportData struct {
	// Component identifies the component
	Component v201.Component `json:"component" validate:"required"`

	// Variable identifies the variable
	Variable v201.Variable `json:"variable" validate:"required"`

	// VariableAttribute contains the attributes of the variable
	VariableAttribute []VariableAttribute `json:"variableAttribute" validate:"required,min=1,max=4,dive"`

	// VariableCharacteristics describes the characteristics
	VariableCharacteristics *VariableCharacteristics `json:"variableCharacteristics,omitempty"`
}

// NotifyReportRequest represents the request for NotifyReport
type NotifyReportRequest struct {
	// RequestId is the ID of the GetBaseReportRequest, GetReport or GetVariables request that requested this report
	RequestId int `json:"requestId" validate:"required"`

	// GeneratedAt is the timestamp of report generation
	GeneratedAt time.Time `json:"generatedAt" validate:"required"`

	// ReportData contains the report data
	ReportData []ReportData `json:"reportData,omitempty" validate:"omitempty,dive"`

	// SeqNo is the sequence number of this message (0-based)
	SeqNo int `json:"seqNo" validate:"required,min=0"`

	// Tbc indicates if another part of the report follows
	Tbc bool `json:"tbc"`
}

// NotifyReportResponse represents the response to NotifyReport
type NotifyReportResponse struct {
	// No fields required - empty response indicates acceptance
}

// GetFeatureName implements common.Request interface
func (r NotifyReportRequest) GetFeatureName() string {
	return NotifyReportFeatureName
}

// GetProtocolVersion implements common.Request interface
func (r NotifyReportRequest) GetProtocolVersion() common.ProtocolVersion {
	return common.OCPP201
}

// Validate implements common.Request interface
func (r NotifyReportRequest) Validate() error {
	if r.SeqNo < 0 {
		return &ValidationError{Field: "seqNo", Message: "must be >= 0"}
	}
	return nil
}

// GetFeatureName implements common.Response interface
func (r NotifyReportResponse) GetFeatureName() string {
	return NotifyReportFeatureName
}

// GetProtocolVersion implements common.Response interface
func (r NotifyReportResponse) GetProtocolVersion() common.ProtocolVersion {
	return common.OCPP201
}
