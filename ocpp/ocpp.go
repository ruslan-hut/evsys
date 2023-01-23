package ocpp

import (
	"encoding/json"
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

func ParseRawJsonRequest(raw interface{}, requestType reflect.Type) (Request, error) {
	if raw == nil {
		raw = &struct{}{}
	}
	bytes, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	request := reflect.New(requestType).Interface()
	err = json.Unmarshal(bytes, &request)
	if err != nil {
		return nil, err
	}
	result := request.(Request)
	return result, nil
}
