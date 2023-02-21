package internal

const FeatureLogMessageType = "featureLogMessage"

type FeatureLogMessage struct {
	Time          string `json:"time" bson:"time"`
	Feature       string `json:"feature" bson:"feature"`
	ChargePointId string `json:"id" bson:"charge_point_id"`
	Text          string `json:"text" bson:"text"`
}

func (fm *FeatureLogMessage) MessageType() string {
	return FeatureLogMessageType
}

func (fm *FeatureLogMessage) DataType() string {
	return FeatureLogMessageType
}
