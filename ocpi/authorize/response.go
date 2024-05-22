package authorize

import "encoding/json"

// Response is the standard response from the OCPI server
type Response struct {
	Data          *Data  `json:"data"`
	StatusCode    int    `json:"status_code"`
	StatusMessage string `json:"status_message"`
	Timestamp     string `json:"timestamp"`
}

type Data struct {
	Status string `json:"status"`
	Info   string `json:"info"`
}

func ParseResponse(body []byte) *Data {
	res := &Response{}
	err := json.Unmarshal(body, res)
	if err != nil {
		return nil
	}
	if res.StatusCode != 1000 {
		return nil
	}
	return res.Data
}
