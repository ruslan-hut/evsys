package authorize

import (
	"evsys/ocpi/client"
	"time"
)

const authorizeEndpoint = "/authorize"

type Authorize struct {
	client *client.Client
}

func New(client *client.Client) *Authorize {
	return &Authorize{
		client: client,
	}
}

func (a *Authorize) Authorize(locationId, evseId, idTag string) *Result {
	req := &Request{
		LocationId: locationId,
		Evse:       evseId,
		IdTag:      idTag,
	}

	rc := make(chan struct {
		body []byte
		err  error
	}, 1)

	a.client.POST(authorizeEndpoint, req, func(body []byte, err error) {
		rc <- struct {
			body []byte
			err  error
		}{body, err}
	})

	select {
	case res := <-rc:
		if res.err == nil {
			response := ParseResponse(res.body)
			return NewFromResponse(response)
		}
	case <-time.After(5 * time.Second):
		return nil
	}
	return nil
}
