package core

const GetConfigurationFeatureName = "GetConfiguration"

// ConfigurationKey Contains information about a specific configuration key. It is returned in GetConfigurationConfirmation
type ConfigurationKey struct {
	Key      string  `json:"key" validate:"required,max=50"`
	Readonly bool    `json:"readonly"`
	Value    *string `json:"value,omitempty" validate:"max=500"`
}

// GetConfigurationRequest The field definition of the GetConfiguration request payload sent by the Central System to the Charge Point.
type GetConfigurationRequest struct {
	Key []string `json:"key,omitempty" validate:"omitempty,unique,dive,max=50"`
}

type GetConfigurationResponse struct {
	ConfigurationKey []ConfigurationKey `json:"configurationKey,omitempty" validate:"omitempty,dive"`
	UnknownKey       []string           `json:"unknownKey,omitempty" validate:"omitempty,dive,max=50"`
}

func (request *GetConfigurationRequest) GetFeatureName() string {
	return GetConfigurationFeatureName
}

func (response *GetConfigurationResponse) GetFeatureName() string {
	return GetConfigurationFeatureName
}

func NewGetConfigurationRequest(key []string) *GetConfigurationRequest {
	return &GetConfigurationRequest{Key: key}
}
