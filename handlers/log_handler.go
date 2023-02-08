package handlers

type LogHandler interface {
	FeatureEvent(feature, id, text string)
}
