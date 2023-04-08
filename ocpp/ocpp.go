package ocpp

import (
	"reflect"
)

// Request message
type Request interface {
	// GetFeatureName Returns the unique name of the feature, to which this request belongs to.
	GetFeatureName() string
}

// Response message
type Response interface {
	// GetFeatureName Returns the unique name of the feature, to which this request belongs to.
	GetFeatureName() string
}

type Feature interface {
	GetFeatureName() string
	GetRequestType() reflect.Type
	GetResponseType() reflect.Type
}
