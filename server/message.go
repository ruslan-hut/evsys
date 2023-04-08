package server

import (
	"encoding/json"
	"evsys/ocpp"
	"evsys/ocpp/core"
	"evsys/ocpp/firmware"
	"fmt"
	"log"
	"math/rand"
	"reflect"
)

//type MessageType string
//
//const (
//	BootNotification MessageType = "BootNotification"
//)

type Message struct {
	Fields []interface{}
}

//func (m *Message) Type() (t MessageType, err error) {
//	if len(m.Fields) < 4 {
//		err = utility.Err("incompatible message structure")
//		return t, err
//	}
//	v := fmt.Sprintf("%v", m.Fields[2])
//	switch v {
//	case string(BootNotification):
//		t = BootNotification
//
//	default:
//		err = utility.Err(fmt.Sprintf("unsupported message type %s", v))
//	}
//	return t, err
//}

//func (m *Message) UniqueId() (id string, err error) {
//	if len(m.Fields) < 4 {
//		err = utility.Err("incompatible message structure")
//		return id, err
//	}
//	id = fmt.Sprintf("%v", m.Fields[1])
//	return id, err
//}

type CallType int

const (
	CallTypeRequest CallType = 2
	CallTypeResult  CallType = 3
	CallTypeError   CallType = 4
)

// CallResult An OCPP-J CallResult message, containing an OCPP Response.
type CallResult struct {
	TypeId   CallType
	UniqueId string
	Payload  *ocpp.Response
}

func (callResult *CallResult) MarshalJSON() ([]byte, error) {
	fields := make([]interface{}, 3)
	fields[0] = int(callResult.TypeId)
	fields[1] = callResult.UniqueId
	fields[2] = callResult.Payload
	return json.Marshal(fields)
}

func CreateCallResult(confirmation *ocpp.Response, uniqueId string) (*CallResult, error) {
	callResult := CallResult{
		TypeId:   CallTypeResult,
		UniqueId: uniqueId,
		Payload:  confirmation,
	}
	return &callResult, nil
}

type CallRequest struct {
	TypeId   CallType
	UniqueId string
	feature  string
	Payload  ocpp.Request
}

func CreateCallRequest(request ocpp.Request) (CallRequest, error) {
	callRequest := CallRequest{
		TypeId:   CallTypeRequest,
		UniqueId: "",
		feature:  request.GetFeatureName(),
		Payload:  request,
	}
	return callRequest, nil
}

func (callRequest *CallRequest) GetFeatureName() string {
	return callRequest.feature
}

func (callRequest *CallRequest) MarshalJSON() ([]byte, error) {
	fields := make([]interface{}, 3)
	fields[0] = int(callRequest.TypeId)
	fields[1] = callRequest.UniqueId
	fields[2] = callRequest.feature
	fields[3] = callRequest.Payload
	return json.Marshal(fields)
}

func ParseMessage(data []interface{}) (*CallRequest, error) {
	if len(data) != 4 {
		return nil, fmt.Errorf("unsupported request format; expected length: 4 elements")
	}
	rawTypeId, ok := data[0].(float64)
	if !ok {
		return nil, fmt.Errorf("invalid message type in request")
	}
	typeId := CallType(rawTypeId)
	if typeId != CallTypeRequest {
		return nil, fmt.Errorf("invalid request type id: %v", typeId)
	}
	uniqueId := data[1].(string)
	if !ok {
		return nil, fmt.Errorf("invalid message unique id in request")
	}
	action := data[2].(string)

	requestType, err := getMessageType(action)
	if err != nil {
		return nil, err
	}
	request, err := ParseRawJsonRequest(data[3], requestType)
	if err != nil {
		return nil, err
	}
	callRequest := CallRequest{
		TypeId:   typeId,
		UniqueId: uniqueId,
		feature:  action,
		Payload:  request,
	}
	return &callRequest, nil
}

func getMessageType(action string) (requestType reflect.Type, err error) {
	switch action {
	case core.BootNotificationFeatureName:
		requestType = reflect.TypeOf(core.BootNotificationRequest{})
	case core.AuthorizeFeatureName:
		requestType = reflect.TypeOf(core.AuthorizeRequest{})
	case core.HeartbeatFeatureName:
		requestType = reflect.TypeOf(core.HeartbeatRequest{})
	case core.StartTransactionFeatureName:
		requestType = reflect.TypeOf(core.StartTransactionRequest{})
	case core.StopTransactionFeatureName:
		requestType = reflect.TypeOf(core.StopTransactionRequest{})
	case core.MeterValuesFeatureName:
		requestType = reflect.TypeOf(core.MeterValuesRequest{})
	case core.StatusNotificationFeatureName:
		requestType = reflect.TypeOf(core.StatusNotificationRequest{})
	case core.DataTransferFeatureName:
		requestType = reflect.TypeOf(core.DataTransferRequest{})
	case firmware.DiagnosticsStatusNotificationFeatureName:
		requestType = reflect.TypeOf(firmware.DiagnosticsStatusNotificationRequest{})
	case firmware.StatusNotificationFeatureName:
		requestType = reflect.TypeOf(firmware.StatusNotificationRequest{})
	default:
		return nil, fmt.Errorf("unsupported action requested: %s", action)
	}
	return requestType, nil
}

func ParseRawJsonRequest(raw interface{}, requestType reflect.Type) (ocpp.Request, error) {
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
		log.Printf("bytes: %v", string(bytes))
		return nil, err
	}
	result := request.(ocpp.Request)
	return result, nil
}

var messageIdGenerator = func() string {
	return fmt.Sprintf("%v", rand.Uint32())
}
