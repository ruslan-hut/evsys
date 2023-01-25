package ocpp

import (
	"evsys/types"
	"log"
	"time"
)

const defaultHeartbeatInterval = 600

type MessageHandler struct {
}

func (h *MessageHandler) OnBootNotification(chargePointId string, request *BootNotificationRequest) (confirmation *BootNotificationResponse, err error) {
	log.Printf("boot confirmed: ID %s; Serial number: %s", chargePointId, request.ChargePointSerialNumber)
	return NewBootNotificationResponse(types.NewDateTime(time.Now()), defaultHeartbeatInterval, RegistrationStatusAccepted), nil
}

func (h *MessageHandler) OnAuthorize(chargePointId string, request *AuthorizeRequest) (confirmation *AuthorizeResponse, err error) {
	log.Printf("authorization accepted: ID %s", chargePointId)
	return NewAuthorizationResponse(types.NewIdTagInfo(types.AuthorizationStatusAccepted)), nil
}
