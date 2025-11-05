package provisioning

import (
	"evsys/ocpp/common"
	"evsys/ocpp/v201"
)

// ============================================================================
// GetBaseReport - OCPP 2.0.1
// ============================================================================
// Sent by: CSMS → Charging Station
// Purpose: Request the Charging Station to send a report of its
//          configuration or status.
// ============================================================================

const GetBaseReportFeatureName = "GetBaseReport"

// GetBaseReportRequest represents the request for GetBaseReport
type GetBaseReportRequest struct {
	// RequestId is the ID of the request
	RequestId int `json:"requestId" validate:"required"`

	// ReportBase specifies the type of report
	ReportBase ReportBaseType `json:"reportBase" validate:"required"`
}

// GenericDeviceModelStatusType defines the status of a device model operation
type GenericDeviceModelStatusType string

const (
	GenericDeviceModelStatusAccepted       GenericDeviceModelStatusType = "Accepted"       // Request accepted
	GenericDeviceModelStatusRejected       GenericDeviceModelStatusType = "Rejected"       // Request rejected
	GenericDeviceModelStatusNotSupported   GenericDeviceModelStatusType = "NotSupported"   // Feature not supported
	GenericDeviceModelStatusEmptyResultSet GenericDeviceModelStatusType = "EmptyResultSet" // No data available
)

// GetBaseReportResponse represents the response to GetBaseReport
type GetBaseReportResponse struct {
	// Status indicates whether the request was accepted
	Status GenericDeviceModelStatusType `json:"status" validate:"required"`

	// StatusInfo provides additional status information
	StatusInfo *v201.StatusInfo `json:"statusInfo,omitempty"`
}

// GetFeatureName implements common.Request interface
func (r GetBaseReportRequest) GetFeatureName() string {
	return GetBaseReportFeatureName
}

// GetProtocolVersion implements common.Request interface
func (r GetBaseReportRequest) GetProtocolVersion() common.ProtocolVersion {
	return common.OCPP201
}

// Validate implements common.Request interface
func (r GetBaseReportRequest) Validate() error {
	if r.ReportBase == "" {
		return &ValidationError{Field: "reportBase", Message: "required"}
	}
	return nil
}

// GetFeatureName implements common.Response interface
func (r GetBaseReportResponse) GetFeatureName() string {
	return GetBaseReportFeatureName
}

// GetProtocolVersion implements common.Response interface
func (r GetBaseReportResponse) GetProtocolVersion() common.ProtocolVersion {
	return common.OCPP201
}
