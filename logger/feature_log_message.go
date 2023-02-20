package logger

const FeatureLogMessageType = "featureLogMessage"

type FeatureLogMessage struct {
	Time          string `json:"time"`
	Feature       string `json:"feature"`
	ChargePointId string `json:"id"`
	Text          string `json:"text"`
}

func (fm *FeatureLogMessage) MessageType() string {
	return FeatureLogMessageType
}

func (fm *FeatureLogMessage) DataType() string {
	return FeatureLogMessageType
}
