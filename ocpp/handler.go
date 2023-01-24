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
