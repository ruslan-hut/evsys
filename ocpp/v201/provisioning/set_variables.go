package provisioning

import (
	"evsys/ocpp/common"
	"evsys/ocpp/v201"
)

// ============================================================================
// SetVariables - OCPP 2.0.1
// ============================================================================
// Sent by: CSMS → Charging Station
// Purpose: Request the Charging Station to set the value of one or more
//          variables in its device model. This replaces ChangeConfiguration
//          from OCPP 1.6.
// ============================================================================

const SetVariablesFeatureName = "SetVariables"

// SetVariableDataType represents a request to set a variable
type SetVariableDataType struct {
	// AttributeType specifies which attribute to set (optional, defaults to Actual)
	AttributeType AttributeType `json:"attributeType,omitempty"`

	// AttributeValue is the value to set
	AttributeValue string `json:"attributeValue" validate:"required,max=1000"`

	// Component identifies the component
	Component v201.Component `json:"component" validate:"required"`

	// Variable identifies the variable
	Variable v201.Variable `json:"variable" validate:"required"`
}

// SetVariablesRequest represents the request for SetVariables
type SetVariablesRequest struct {
	// SetVariableData contains the variables to set
	SetVariableData []SetVariableDataType `json:"setVariableData" validate:"required,min=1,dive"`
}

// SetVariableStatusType defines the status of setting a variable
type SetVariableStatusType string

const (
	SetVariableStatusAccepted                  SetVariableStatusType = "Accepted"                  // Variable set successfully
	SetVariableStatusRejected                  SetVariableStatusType = "Rejected"                  // Request rejected
	SetVariableStatusUnknownComponent          SetVariableStatusType = "UnknownComponent"          // Component unknown
	SetVariableStatusUnknownVariable           SetVariableStatusType = "UnknownVariable"           // Variable unknown
	SetVariableStatusNotSupportedAttributeType SetVariableStatusType = "NotSupportedAttributeType" // Attribute type not supported
	SetVariableStatusRebootRequired            SetVariableStatusType = "RebootRequired"            // Reboot required for change to take effect
)

// SetVariableResultType represents the result of setting a variable
type SetVariableResultType struct {
	// AttributeType specifies which attribute was set
	AttributeType AttributeType `json:"attributeType,omitempty"`

	// AttributeStatus indicates the result status
	AttributeStatus SetVariableStatusType `json:"attributeStatus" validate:"required"`

	// Component identifies the component
	Component v201.Component `json:"component" validate:"required"`

	// Variable identifies the variable
	Variable v201.Variable `json:"variable" validate:"required"`

	// AttributeStatusInfo provides additional status information
	AttributeStatusInfo *v201.StatusInfo `json:"attributeStatusInfo,omitempty"`
}

// SetVariablesResponse represents the response to SetVariables
type SetVariablesResponse struct {
	// SetVariableResult contains the results
	SetVariableResult []SetVariableResultType `json:"setVariableResult" validate:"required,min=1,dive"`
}

// GetFeatureName implements common.Request interface
func (r SetVariablesRequest) GetFeatureName() string {
	return SetVariablesFeatureName
}

// GetProtocolVersion implements common.Request interface
func (r SetVariablesRequest) GetProtocolVersion() common.ProtocolVersion {
	return common.OCPP201
}

// Validate implements common.Request interface
func (r SetVariablesRequest) Validate() error {
	if len(r.SetVariableData) == 0 {
		return &ValidationError{Field: "setVariableData", Message: "at least one variable required"}
	}
	for i, data := range r.SetVariableData {
		if data.AttributeValue == "" {
			return &ValidationError{Field: "setVariableData[" + string(rune(i)) + "].attributeValue", Message: "required"}
		}
	}
	return nil
}

// GetFeatureName implements common.Response interface
func (r SetVariablesResponse) GetFeatureName() string {
	return SetVariablesFeatureName
}

// GetProtocolVersion implements common.Response interface
func (r SetVariablesResponse) GetProtocolVersion() common.ProtocolVersion {
	return common.OCPP201
}
