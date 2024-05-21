package listener

import (
	"bytes"
	"context"
	"encoding/json"
	"evsys/internal"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

const (
	startEndpoint  = "/transactions/start"
	stopEndpoint   = "/transactions/stop"
	statusEndpoint = "/status"
)

type Listener struct {
	client *http.Client
	url    string
	token  string
}

func New(url, token string) *Listener {
	return &Listener{
		url:    url,
		token:  token,
		client: &http.Client{},
	}
}

func (l *Listener) OnStatusNotification(event *internal.EventMessage) {
	url := fmt.Sprintf("%v%v", l.url, statusEndpoint)
	l.postEvent(url, event)
}

func (l *Listener) OnTransactionStart(event *internal.EventMessage) {
	url := fmt.Sprintf("%v%v", l.url, startEndpoint)
	l.postEvent(url, event)
}

func (l *Listener) OnTransactionStop(event *internal.EventMessage) {
	url := fmt.Sprintf("%v%v", l.url, stopEndpoint)
	l.postEvent(url, event)
}

func (l *Listener) OnAuthorize(_ *internal.EventMessage) {

}

func (l *Listener) OnTransactionEvent(_ *internal.EventMessage) {

}

func (l *Listener) OnAlert(_ *internal.EventMessage) {

}

func (l *Listener) OnInfo(_ *internal.EventMessage) {

}

func (l *Listener) postEvent(url string, event *internal.EventMessage) {
	body, err := json.Marshal(event)
	if err != nil {
		log.Printf("listener: error marshalling event: %v", err)
		return
	}
	go func() {
		for attempt := 0; attempt < 5; attempt++ {
			err = l.doRequest(url, body)
			if err == nil {
				return
			}
			log.Printf("listener: %s: %v (attempt %d)", url, err, attempt+1)
			time.Sleep(time.Duration((attempt+1)*10) * time.Second)
		}
	}()
}

func (l *Listener) doRequest(url string, body []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Token "+l.token)

	resp, err := l.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-200 status code: %d", resp.StatusCode)
	}
	return nil
}
