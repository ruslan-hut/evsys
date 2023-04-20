package localauth

import (
	"evsys/types"
	"reflect"
)

const SendLocalListFeatureName = "SendLocalList"

type UpdateType string
type UpdateStatus string

const (
	UpdateTypeDifferential      UpdateType   = "Differential"
	UpdateTypeFull              UpdateType   = "Full"
	UpdateStatusAccepted        UpdateStatus = "Accepted"
	UpdateStatusFailed          UpdateStatus = "Failed"
	UpdateStatusNotSupported    UpdateStatus = "NotSupported"
	UpdateStatusVersionMismatch UpdateStatus = "VersionMismatch"
)

type AuthorizationData struct {
	IdTag     string           `json:"idTag" validate:"required,max=20"`
	IdTagInfo *types.IdTagInfo `json:"idTagInfo,omitempty"` //TODO: validate required if update type is Full
}

type SendLocalListRequest struct {
	ListVersion            int                 `json:"listVersion" validate:"gte=0"`
	LocalAuthorizationList []AuthorizationData `json:"localAuthorizationList,omitempty" validate:"omitempty,dive"`
	UpdateType             UpdateType          `json:"updateType" validate:"required,updateType"`
}

type SendLocalListResponse struct {
	Status UpdateStatus `json:"status" validate:"required,updateStatus"`
}

type SendLocalListFeature struct{}

func (f SendLocalListFeature) GetFeatureName() string {
	return SendLocalListFeatureName
}

func (f SendLocalListFeature) GetRequestType() reflect.Type {
	return reflect.TypeOf(SendLocalListRequest{})
}

func (f SendLocalListFeature) GetResponseType() reflect.Type {
	return reflect.TypeOf(SendLocalListResponse{})
}

func (r SendLocalListRequest) GetFeatureName() string {
	return SendLocalListFeatureName
}

func (c SendLocalListResponse) GetFeatureName() string {
	return SendLocalListFeatureName
}

// NewSendLocalListRequest creates SendLocalListRequest containing all required field. Optional fields may be set afterward.
func NewSendLocalListRequest(version int, updateType UpdateType) *SendLocalListRequest {
	return &SendLocalListRequest{ListVersion: version, UpdateType: updateType}
}

// NewSendLocalListResponse Creates a new SendLocalListConfirmation, containing all required fields. There are no optional fields for this message.
func NewSendLocalListResponse(status UpdateStatus) *SendLocalListResponse {
	return &SendLocalListResponse{Status: status}
}
