package ocpp

import "evsys/types"

const AuthorizeFeatureName = "Authorize"

type AuthorizeRequest struct {
	IdTag string `json:"idTag" validate:"required,max=20"`
}

func (f *AuthorizeRequest) GetFeatureName() string {
	return AuthorizeFeatureName
}

type AuthorizeResponse struct {
	IdTagInfo *types.IdTagInfo `json:"idTagInfo" validate:"required"`
}

func (f *AuthorizeResponse) GetFeatureName() string {
	return AuthorizeFeatureName
}

type AuthorizeFeature struct {
}

func (f *AuthorizeFeature) GetFeatureName() string {
	return AuthorizeFeatureName
}

func NewAuthorizationResponse(idTagInfo *types.IdTagInfo) *AuthorizeResponse {
	return &AuthorizeResponse{IdTagInfo: idTagInfo}
}
