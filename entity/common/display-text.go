package common

// Language Code ISO 639-1
// Text to be displayed to end user. No markup, html etc. allowed.

type DisplayText struct {
	Language string `json:"language" bson:"language" validate:"required,max=2"`
	Text     string `json:"text" bson:"text" validate:"required,min=1,max=512"`
}
