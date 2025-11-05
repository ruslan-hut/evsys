package core

const ChangeConfigurationFeatureName = "ChangeConfiguration"

type ConfigurationStatus string

//const (
//	ConfigurationStatusAccepted       ConfigurationStatus = "Accepted"
//	ConfigurationStatusRejected       ConfigurationStatus = "Rejected"
//	ConfigurationStatusRebootRequired ConfigurationStatus = "RebootRequired"
//	ConfigurationStatusNotSupported   ConfigurationStatus = "NotSupported"
//)

type ChangeConfigurationRequest struct {
	Key   string `json:"key" validate:"required,max=50"`
	Value string `json:"value" validate:"required,max=500"`
}

type ChangeConfigurationResponse struct {
	Status ConfigurationStatus `json:"status" validate:"required,configurationStatus"`
}

func (request *ChangeConfigurationRequest) GetFeatureName() string {
	return ChangeConfigurationFeatureName
}

func (response *ChangeConfigurationResponse) GetFeatureName() string {
	return ChangeConfigurationFeatureName
}
