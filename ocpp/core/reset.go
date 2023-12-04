package core

const ResetFeatureName = "Reset"

type ResetType string

//type ResetStatus string

//const (
//	ResetTypeHard ResetType = "Hard"
//	ResetTypeSoft ResetType = "Soft"
//	//ResetStatusAccepted ResetStatus = "Accepted"
//	//ResetStatusRejected ResetStatus = "Rejected"
//)

type ResetRequest struct {
	Type ResetType `json:"type" validate:"required,resetType"`
}

func NewResetRequest(resetType ResetType) *ResetRequest {
	return &ResetRequest{Type: resetType}
}

func (r *ResetRequest) GetFeatureName() string {
	return ResetFeatureName
}
