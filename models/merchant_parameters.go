package models

type MerchantParameters struct {
	Amount          string `json:"DS_MERCHANT_AMOUNT"`
	Order           string `json:"DS_MERCHANT_ORDER"`
	Identifier      string `json:"DS_MERCHANT_IDENTIFIER"`
	MerchantCode    string `json:"DS_MERCHANT_MERCHANTCODE"`
	Currency        string `json:"DS_MERCHANT_CURRENCY"`
	TransactionType string `json:"DS_MERCHANT_TRANSACTIONTYPE"`
	Terminal        string `json:"DS_MERCHANT_TERMINAL"`
	DirectPayment   string `json:"DS_MERCHANT_DIRECTPAYMENT"`
	Exception       string `json:"DS_MERCHANT_EXCEP_SCA"`
	Cof             string `json:"DS_MERCHANT_COF_INI"`
}
