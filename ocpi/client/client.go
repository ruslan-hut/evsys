package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type Client struct {
	client *http.Client
	url    string
	token  string
}

func New(url, token string) *Client {
	return &Client{
		url:    url,
		token:  token,
		client: &http.Client{},
	}
}

func (c *Client) POST(endpoint string, data interface{}, callback func(resp []byte, err error)) {
	body, err := json.Marshal(data)
	if err != nil {
		log.Printf("ocpi client: error marshalling body: %v", err)
		return
	}
	go func() {
		var resp []byte
		for attempt := 0; attempt < 3; attempt++ {
			resp, err = c.doRequest(endpoint, body)
			if err == nil {
				callback(resp, nil)
				return
			}
			log.Printf("ocpi client: %s: %v (attempt %d)", endpoint, err, attempt+1)
			time.Sleep(time.Duration((attempt+1)*10) * time.Second)
		}
		callback(nil, err)
	}()
}

func (c *Client) doRequest(endpoint string, body []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := fmt.Sprintf("%v%v", c.url, endpoint)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Token "+c.token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-200 status code: %d", resp.StatusCode)
	}

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}
	return body, nil
}
