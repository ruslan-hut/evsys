package types

const SubProtocol16 = "ocpp1.6"

type AuthorizationStatus string

const (
	AuthorizationStatusAccepted     AuthorizationStatus = "Accepted"
	AuthorizationStatusBlocked      AuthorizationStatus = "Blocked"
	AuthorizationStatusExpired      AuthorizationStatus = "Expired"
	AuthorizationStatusInvalid      AuthorizationStatus = "Invalid"
	AuthorizationStatusConcurrentTx AuthorizationStatus = "ConcurrentTx"
)

type IdTagInfo struct {
	ExpiryDate  *DateTime           `json:"expiryDate,omitempty" validate:"omitempty"`
	ParentIdTag string              `json:"parentIdTag,omitempty" validate:"omitempty,max=20"`
	Status      AuthorizationStatus `json:"status" validate:"required,authorizationStatus"`
}

func NewIdTagInfo(status AuthorizationStatus) *IdTagInfo {
	return &IdTagInfo{Status: status}
}
