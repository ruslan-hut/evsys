package pusher

type FeatureLogMessage struct {
	Time          string `json:"time"`
	Feature       string `json:"feature"`
	ChargePointId string `json:"id"`
	Text          string `json:"text"`
}
