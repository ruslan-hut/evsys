package entity

type PaymentMethod struct {
	Description string `json:"description" bson:"description"`
	Identifier  string `json:"identifier" bson:"identifier"`
	CardNumber  string `json:"card_number" bson:"card_number"`
	CardType    string `json:"card_type" bson:"card_type"`
	CardBrand   string `json:"card_brand" bson:"card_brand"`
	CardCountry string `json:"card_country" bson:"card_country"`
	ExpiryDate  string `json:"expiry_date" bson:"expiry_date"`
	IsDefault   bool   `json:"is_default" bson:"is_default"`
	UserId      string `json:"user_id" bson:"user_id"`
	UserName    string `json:"user_name" bson:"user_name"`
	FailCount   int    `json:"fail_count" bson:"fail_count"`
	CofTid      string `json:"merchant_cof_txnid" bson:"merchant_cof_txnid"`
}
