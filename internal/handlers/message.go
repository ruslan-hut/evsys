package handlers

import (
	"encoding/json"
	"evsys/internal/ocpp16/messages"
	"evsys/ocpp"
	"evsys/utility"
	"fmt"
	"log"
)

type MessageType string

const (
	BootNotification MessageType = "BootNotification"
)

type Message struct {
	Fields []interface{}
}

func (m *Message) Type() (t MessageType, err error) {
	if len(m.Fields) < 4 {
		err = utility.Err("incompatible message structure")
		return t, err
	}
	v := fmt.Sprintf("%v", m.Fields[2])
	switch v {
	case string(BootNotification):
		t = BootNotification

	default:
		err = utility.Err(fmt.Sprintf("unsupported message type %s", v))
	}
	return t, err
}

func (m *Message) UniqueId() (id string, err error) {
	if len(m.Fields) < 4 {
		err = utility.Err("incompatible message structure")
		return id, err
	}
	id = fmt.Sprintf("%v", m.Fields[1])
	return id, err
}

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
	Feature  string
	Payload  ocpp.Request
}

func ParseRequest(data []interface{}) (*CallRequest, error) {
	if len(data) != 4 {
		return nil, utility.Err("unsupported request format; expected length: 4 elements")
	}
	rawTypeId, ok := data[0].(float64)
	if !ok {
		return nil, utility.Err("invalid message type in request")
	}
	typeId := CallType(rawTypeId)
	if typeId != CallTypeRequest {
		return nil, utility.Err(fmt.Sprintf("invalid request type id: %v", typeId))
	}
	uniqueId := data[1].(string)
	if !ok {
		return nil, utility.Err("invalid message unique id in request")
	}
	action := data[2].(string)
	log.Printf("<<< got request %s (%s)", action, uniqueId)

	if action == string(BootNotification) {
		bootRequest := messages.BootNotificationRequest{}
		request, err := ocpp.ParseRawJsonRequest(data[3], bootRequest.GetRequestType())
		if err != nil {
			return nil, err
		}
		callRequest := CallRequest{
			TypeId:   typeId,
			UniqueId: uniqueId,
			Feature:  action,
			Payload:  request,
		}
		return &callRequest, nil
	}
	return nil, utility.Err(fmt.Sprintf("unsupported action requested: %s", action))
}
