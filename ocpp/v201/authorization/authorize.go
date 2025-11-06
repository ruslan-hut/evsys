package authorization

import (
	"evsys/ocpp/common"
	"evsys/ocpp/v201"
)

// ============================================================================
// Authorize - OCPP 2.0.1
// ============================================================================
// Sent by: Charging Station → CSMS
// Purpose: Request authorization for an IdToken. This is the OCPP 2.0.1
//          equivalent of the OCPP 1.6 Authorize request, but uses IdToken
//          instead of IdTag.
// ============================================================================

const AuthorizeFeatureName = "Authorize"

// AuthorizeRequest represents the request for Authorize
type AuthorizeRequest struct {
	// IdToken is the identifier that needs to be authorized
	IdToken v201.IdToken `json:"idToken" validate:"required"`

	// Certificate is the X.509 certificate presented by EV (ISO 15118)
	Certificate string `json:"certificate,omitempty" validate:"omitempty,max=5500"`

	// Iso15118CertificateHashData contains hashed certificate info
	Iso15118CertificateHashData []v201.OCSPRequestDataType `json:"iso15118CertificateHashData,omitempty" validate:"omitempty,max=4,dive"`
}

// AuthorizeResponse represents the response to Authorize
type AuthorizeResponse struct {
	// IdTokenInfo contains authorization information
	IdTokenInfo v201.IdTokenInfo `json:"idTokenInfo" validate:"required"`

	// CertificateStatus indicates the certificate validation result (for ISO 15118)
	CertificateStatus v201.AuthorizeCertificateStatusType `json:"certificateStatus,omitempty"`
}

// GetFeatureName implements common.Request interface
func (r AuthorizeRequest) GetFeatureName() string {
	return AuthorizeFeatureName
}

// GetProtocolVersion implements common.Request interface
func (r AuthorizeRequest) GetProtocolVersion() common.ProtocolVersion {
	return common.OCPP201
}

// Validate implements common.Request interface
func (r AuthorizeRequest) Validate() error {
	if err := r.IdToken.Validate(); err != nil {
		return err
	}
	if len(r.Certificate) > 5500 {
		return &ValidationError{Field: "certificate", Message: "max 5500 characters"}
	}
	if len(r.Iso15118CertificateHashData) > 4 {
		return &ValidationError{Field: "iso15118CertificateHashData", Message: "max 4 items"}
	}
	return nil
}

// GetFeatureName implements common.Response interface
func (r AuthorizeResponse) GetFeatureName() string {
	return AuthorizeFeatureName
}

// GetProtocolVersion implements common.Response interface
func (r AuthorizeResponse) GetProtocolVersion() common.ProtocolVersion {
	return common.OCPP201
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
