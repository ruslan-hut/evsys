package server

import (
	"crypto/tls"
	"evsys/internal"
	"evsys/internal/config"
	"fmt"
	"net/http"
)

const (
	apiEndpoint = "/api"
)

type Api struct {
	conf       *config.Config
	httpServer *http.Server
	logger     internal.LogHandler
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
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("hello world"))
}
