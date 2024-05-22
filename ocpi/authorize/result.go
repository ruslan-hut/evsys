package authorize

type Result struct {
	Allowed bool
	Expired bool
	Blocked bool
	Info    string
}

func NewFromResponse(response *Response) *Result {
	return &Result{
		Allowed: response.Status == "ALLOWED",
		Expired: response.Status == "EXPIRED",
		Blocked: response.Status == "BLOCKED",
		Info:    response.Info,
	}
}
