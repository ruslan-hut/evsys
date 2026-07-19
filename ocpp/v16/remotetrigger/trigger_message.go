package remotetrigger

const TriggerMessageFeatureName = "TriggerMessage"

type MessageTrigger string

type TriggerMessageStatus string

const (
	TriggerMessageStatusAccepted       TriggerMessageStatus = "Accepted"
	TriggerMessageStatusRejected       TriggerMessageStatus = "Rejected"
	TriggerMessageStatusNotImplemented TriggerMessageStatus = "NotImplemented"
)

type TriggerMessageRequest struct {
	RequestedMessage MessageTrigger `json:"requestedMessage" validate:"required,messageTrigger"`
	ConnectorId      *int           `json:"connectorId,omitempty" validate:"omitempty,gt=0"`
}

func (f TriggerMessageRequest) GetFeatureName() string {
	return TriggerMessageFeatureName
}

func NewTriggerMessageRequest(requestedMessage MessageTrigger, connectorId int) *TriggerMessageRequest {
	request := &TriggerMessageRequest{RequestedMessage: requestedMessage}
	if connectorId >= 0 {
		request.ConnectorId = &connectorId
	}
	return request
}

type TriggerMessageResponse struct {
	Status TriggerMessageStatus `json:"status" validate:"required,triggerMessageStatus"`
}

func (f TriggerMessageResponse) GetFeatureName() string {
	return TriggerMessageFeatureName
}
