package firmware

import "fmt"

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

// IsValid reports whether the status is a known OCPP 1.6 diagnostics status.
func (s DiagnosticsStatus) IsValid() bool {
	switch s {
	case DiagnosticsStatusIdle, DiagnosticsStatusUploaded, DiagnosticsStatusUploadFailed, DiagnosticsStatusUploading:
		return true
	default:
		return false
	}
}

// Validate checks that the status is a known value.
func (r DiagnosticsStatusNotificationRequest) Validate() error {
	if r.Status == "" || !r.Status.IsValid() {
		return fmt.Errorf("invalid status: %q", r.Status)
	}
	return nil
}

func (c DiagnosticsStatusNotificationResponse) GetFeatureName() string {
	return DiagnosticsStatusNotificationFeatureName
}

func NewDiagnosticsStatusNotificationResponse() *DiagnosticsStatusNotificationResponse {
	return &DiagnosticsStatusNotificationResponse{}
}
