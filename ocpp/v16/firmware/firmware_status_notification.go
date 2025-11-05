package firmware

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

func (c StatusNotificationResponse) GetFeatureName() string {
	return StatusNotificationFeatureName
}

func NewStatusNotificationResponse() *StatusNotificationResponse {
	return &StatusNotificationResponse{}
}
