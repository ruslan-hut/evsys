package authorize

import "encoding/json"

type Response struct {
	Status string `json:"status"`
	Info   string `json:"info"`
}

func ParseResponse(body []byte) *Response {
	res := &Response{}
	_ = json.Unmarshal(body, res)
	return res
}
