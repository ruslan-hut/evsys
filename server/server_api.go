package server

import (
	"crypto/tls"
	"encoding/json"
	"evsys/internal"
	"evsys/internal/config"
	"fmt"
	"io"
	"net/http"
)

const (
	apiEndpoint = "/api"
)

type SupportedFeature string

type Api struct {
	conf           *config.Config
	httpServer     *http.Server
	requestHandler func(w http.ResponseWriter, command CentralSystemCommand) error
	logger         internal.LogHandler
}

type apiResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

func NewServerApi(conf *config.Config, logger internal.LogHandler) *Api {
	server := Api{
		conf:   conf,
		logger: logger,
	}
	server.httpServer = &http.Server{
		Addr:    fmt.Sprintf("%s:%s", conf.Api.BindIP, conf.Api.Port),
		Handler: http.HandlerFunc(server.handleRoot),
	}
	return &server
}

func (s *Api) Start() error {
	var err error
	if s.conf.Api.TLS {
		cert, err := tls.LoadX509KeyPair(s.conf.Api.CertFile, s.conf.Api.KeyFile)
		if err != nil {
			return fmt.Errorf("api: failed to load certificate: %v", err)
		}
		s.httpServer.TLSConfig = &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{cert},
		}
		err = s.httpServer.ListenAndServeTLS("", "")
	} else {
		err = s.httpServer.ListenAndServe()
	}
	return err
}

func (s *Api) SetRequestHandler(handler func(w http.ResponseWriter, command CentralSystemCommand) error) {
	s.requestHandler = handler
}

// handle requests to the root path
func (s *Api) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.logger.Warn(fmt.Sprintf("api: invalid method %s from %s", r.Method, r.RemoteAddr))
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path != apiEndpoint {
		s.logger.Warn(fmt.Sprintf("api: invalid path %s from %s", r.URL.Path, r.RemoteAddr))
		w.WriteHeader(http.StatusNotFound)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.logger.Warn(fmt.Sprintf("api: error reading body from %s: %s", r.RemoteAddr, err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// cast body to command
	var cmd CentralSystemCommand
	err = json.Unmarshal(body, &cmd)
	if err != nil {
		s.logger.Warn(fmt.Sprintf("api: error parsing command from %s: %s", r.RemoteAddr, err))
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// send command to websocket
	err = s.requestHandler(w, cmd)
	if err != nil {
		rs := apiResponse{
			Status: "error",
			Error:  err.Error(),
		}
		payload, err := json.Marshal(rs)
		if err != nil {
			s.logger.Warn(fmt.Sprintf("api: error encoding response: %s", err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		//w.Header().Add("Content-Type", "application/json; charset=utf-8")
		_, err = w.Write(payload)
		if err != nil {
			s.logger.Error("api: send response", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}
