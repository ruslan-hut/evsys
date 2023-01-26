package core

import "evsys/types"

const HeartbeatFeatureName = "Heartbeat"

type HeartbeatRequest struct {
}

type HeartbeatResponse struct {
	CurrentTime *types.DateTime `json:"currentTime" validate:"required"`
}

func (req HeartbeatRequest) GetFeatureName() string {
	return HeartbeatFeatureName
}

func (res HeartbeatResponse) GetFeatureName() string {
	return HeartbeatFeatureName
}

func NewHeartbeatResponse(currentTime *types.DateTime) *HeartbeatResponse {
	return &HeartbeatResponse{CurrentTime: currentTime}
}
