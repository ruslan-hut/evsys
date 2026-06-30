package firmware

import "fmt"

const StatusNotificationFeatureName = "FirmwareStatusNotification"

type Status string

const (
	StatusDownloaded         Status = "Downloaded"
	StatusDownloadFailed     Status = "DownloadFailed"
	StatusDownloading        Status = "Downloading"
	StatusIdle               Status = "Idle"
	StatusInstallationFailed Status = "InstallationFailed"
	StatusInstalling         Status = "Installing"
	StatusInstalled          Status = "Installed"
)

type StatusNotificationRequest struct {
	Status Status `json:"status" validate:"required,firmwareStatus"`
}

type StatusNotificationResponse struct {
}

func (r StatusNotificationRequest) GetFeatureName() string {
	return StatusNotificationFeatureName
}

// IsValid reports whether the status is a known OCPP 1.6 firmware status.
func (s Status) IsValid() bool {
	switch s {
	case StatusDownloaded, StatusDownloadFailed, StatusDownloading, StatusIdle,
		StatusInstallationFailed, StatusInstalling, StatusInstalled:
		return true
	default:
		return false
	}
}

// Validate checks that the status is a known value.
func (r StatusNotificationRequest) Validate() error {
	if r.Status == "" || !r.Status.IsValid() {
		return fmt.Errorf("invalid status: %q", r.Status)
	}
	return nil
}

func (c StatusNotificationResponse) GetFeatureName() string {
	return StatusNotificationFeatureName
}

func NewStatusNotificationResponse() *StatusNotificationResponse {
	return &StatusNotificationResponse{}
}
