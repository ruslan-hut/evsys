package provisioning

import (
	"evsys/ocpp/common"
	"evsys/ocpp/v201"
)

// ============================================================================
// GetVariables - OCPP 2.0.1
// ============================================================================
// Sent by: CSMS → Charging Station
// Purpose: Request the Charging Station to report the value of one or more
//          variables from its device model. This replaces GetConfiguration
//          from OCPP 1.6.
// ============================================================================

const GetVariablesFeatureName = "GetVariables"

// GetVariableDataType represents a request to get a variable
type GetVariableDataType struct {
	// Component identifies the component
	Component v201.Component `json:"component" validate:"required"`

	// Variable identifies the variable
	Variable v201.Variable `json:"variable" validate:"required"`

	// AttributeType specifies which attribute to get (optional, defaults to Actual)
	AttributeType AttributeType `json:"attributeType,omitempty"`
}

// GetVariablesRequest represents the request for GetVariables
type GetVariablesRequest struct {
	// GetVariableData contains the variables to get
	GetVariableData []GetVariableDataType `json:"getVariableData" validate:"required,min=1,dive"`
}

// GetVariableStatusType defines the status of getting a variable
type GetVariableStatusType string

const (
	GetVariableStatusAccepted                  GetVariableStatusType = "Accepted"                  // Variable value returned
	GetVariableStatusRejected                  GetVariableStatusType = "Rejected"                  // Request rejected
	GetVariableStatusUnknownComponent          GetVariableStatusType = "UnknownComponent"          // Component unknown
	GetVariableStatusUnknownVariable           GetVariableStatusType = "UnknownVariable"           // Variable unknown
	GetVariableStatusNotSupportedAttributeType GetVariableStatusType = "NotSupportedAttributeType" // Attribute type not supported
)

// GetVariableResultType represents the result of getting a variable
type GetVariableResultType struct {
	// AttributeStatus indicates the result status
	AttributeStatus GetVariableStatusType `json:"attributeStatus" validate:"required"`

	// AttributeType specifies which attribute was retrieved
	AttributeType AttributeType `json:"attributeType,omitempty"`

	// AttributeValue is the value of the attribute
	AttributeValue string `json:"attributeValue,omitempty" validate:"omitempty,max=2500"`

	// Component identifies the component
	Component v201.Component `json:"component" validate:"required"`

	// Variable identifies the variable
	Variable v201.Variable `json:"variable" validate:"required"`

	// AttributeStatusInfo provides additional status information
	AttributeStatusInfo *v201.StatusInfo `json:"attributeStatusInfo,omitempty"`
}

// GetVariablesResponse represents the response to GetVariables
type GetVariablesResponse struct {
	// GetVariableResult contains the results
	GetVariableResult []GetVariableResultType `json:"getVariableResult" validate:"required,min=1,dive"`
}

// GetFeatureName implements common.Request interface
func (r GetVariablesRequest) GetFeatureName() string {
	return GetVariablesFeatureName
}

// GetProtocolVersion implements common.Request interface
func (r GetVariablesRequest) GetProtocolVersion() common.ProtocolVersion {
	return common.OCPP201
}

// Validate implements common.Request interface
func (r GetVariablesRequest) Validate() error {
	if len(r.GetVariableData) == 0 {
		return &ValidationError{Field: "getVariableData", Message: "at least one variable required"}
	}
	return nil
}

// GetFeatureName implements common.Response interface
func (r GetVariablesResponse) GetFeatureName() string {
	return GetVariablesFeatureName
}

// GetProtocolVersion implements common.Response interface
func (r GetVariablesResponse) GetProtocolVersion() common.ProtocolVersion {
	return common.OCPP201
}
