package api

import (
	"encoding/json"
	"evsys/internal"
	"fmt"
)

type CallType string

const (
	ReadLog CallType = "ReadLog"
)

type Call struct {
	CallType CallType
	Remote   string
}

type Handler struct {
	logger   internal.LogHandler
	database internal.Database
}

func (h *Handler) SetLogger(logger internal.LogHandler) {
	h.logger = logger
}

func (h *Handler) SetDatabase(database internal.Database) {
	h.database = database
}

func NewApiHandler() *Handler {
	handler := Handler{}
	return &handler
}

func (h *Handler) HandleApiCall(ac *Call) []byte {
	h.logger.Debug(fmt.Sprintf("api call %s from remote %s", ac.CallType, ac.Remote))
	if h.database == nil {
		return nil
	}
	data, err := h.database.ReadLog()
	if err != nil {
		h.logger.Error("read log error", err)
		return nil
	}
	byteData, err := json.Marshal(data)
	if err != nil {
		h.logger.Error("encoding log data failed", err)
		return nil
	}
	return byteData
}
