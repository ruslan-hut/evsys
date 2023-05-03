package internal

import "time"

const FeatureLogMessageType = "featureLogMessage"

type FeatureLogMessage struct {
	Time          string    `json:"time" bson:"time"`
	TimeStamp     time.Time `json:"timestamp" bson:"timestamp"`
	Feature       string    `json:"feature" bson:"feature"`
	ChargePointId string    `json:"id" bson:"charge_point_id"`
	Text          string    `json:"text" bson:"text"`
	Importance    string    `json:"importance" bson:"importance"`
}

func (fm *FeatureLogMessage) MessageType() string {
	return FeatureLogMessageType
}

func (fm *FeatureLogMessage) DataType() string {
	return FeatureLogMessageType
}
