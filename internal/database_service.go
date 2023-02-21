package internal

type Database interface {
	Write(table string, data Data) error
	WriteLogMessage(data Data) error
	ReadLog() ([]Data, error)
}

type Data interface {
	DataType() string
}
