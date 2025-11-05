package firmware

import "evsys/types"

const GetDiagnosticsFeatureName = "GetDiagnostics"

type GetDiagnosticsRequest struct {
	Location      string          `json:"location" validate:"required,uri"`
	Retries       *int            `json:"retries,omitempty" validate:"omitempty,gte=0"`
	RetryInterval *int            `json:"retryInterval,omitempty" validate:"omitempty,gte=0"`
	StartTime     *types.DateTime `json:"startTime,omitempty"`
	StopTime      *types.DateTime `json:"stopTime,omitempty"`
}

func (r GetDiagnosticsRequest) GetFeatureName() string {
	return GetDiagnosticsFeatureName
}

func NewGetDiagnosticsRequest(location string) *GetDiagnosticsRequest {
	return &GetDiagnosticsRequest{Location: location}
}
