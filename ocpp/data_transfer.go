package ocpp

const DataTransferFeatureName = "DataTransfer"

type DataTransferStatus string

const (
	DataTransferStatusAccepted         DataTransferStatus = "Accepted"
	DataTransferStatusRejected         DataTransferStatus = "Rejected"
	DataTransferStatusUnknownMessageId DataTransferStatus = "UnknownMessageId"
	DataTransferStatusUnknownVendorId  DataTransferStatus = "UnknownVendorId"
)

type DataTransferRequest struct {
	VendorId  string      `json:"vendorId" validate:"required,max=255"`
	MessageId string      `json:"messageId,omitempty" validate:"max=50"`
	Data      interface{} `json:"data,omitempty"`
}

type DataTransferResponse struct {
	Status DataTransferStatus `json:"status" validate:"required,dataTransferStatus"`
	Data   interface{}        `json:"data,omitempty"`
}

func (r DataTransferRequest) GetFeatureName() string {
	return DataTransferFeatureName
}

func (c DataTransferResponse) GetFeatureName() string {
	return DataTransferFeatureName
}

func NewDataTransferResponse(status DataTransferStatus) *DataTransferResponse {
	return &DataTransferResponse{Status: status}
}
