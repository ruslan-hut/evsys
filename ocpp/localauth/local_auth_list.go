package localauth

type SystemHandler interface {
	OnSendLocalList(chargePointId string) (*SendLocalListRequest, error)
}
