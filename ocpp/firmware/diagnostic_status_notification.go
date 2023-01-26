package firmware

const DiagnosticsStatusNotificationFeatureName = "DiagnosticsStatusNotification"

type DiagnosticsStatus string

const (
	DiagnosticsStatusIdle         DiagnosticsStatus = "Idle"
	DiagnosticsStatusUploaded     DiagnosticsStatus = "Uploaded"
	DiagnosticsStatusUploadFailed DiagnosticsStatus = "UploadFailed"
	DiagnosticsStatusUploading    DiagnosticsStatus = "Uploading"
)

type DiagnosticsStatusNotificationRequest struct {
	Status DiagnosticsStatus `json:"status" validate:"required,diagnosticsStatus"`
}

type DiagnosticsStatusNotificationResponse struct {
}

func (r DiagnosticsStatusNotificationRequest) GetFeatureName() string {
	return DiagnosticsStatusNotificationFeatureName
}

func (c DiagnosticsStatusNotificationResponse) GetFeatureName() string {
	return DiagnosticsStatusNotificationFeatureName
}

func NewDiagnosticsStatusNotificationResponse() *DiagnosticsStatusNotificationResponse {
	return &DiagnosticsStatusNotificationResponse{}
}
