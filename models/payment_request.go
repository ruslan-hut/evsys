package models

type PaymentRequest struct {
	Parameters       string `json:"DS_MERCHANT_PARAMETERS"`
	Signature        string `json:"DS_SIGNATURE"`
	SignatureVersion string `json:"DS_SIGNATURE_VERSION"`
}
