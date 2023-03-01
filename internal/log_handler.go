package internal

type LogHandler interface {
	FeatureEvent(feature, id, text string)
	RawDataEvent(direction, data string)
	Debug(text string)
	Warn(text string)
	Error(text string, err error)
}
