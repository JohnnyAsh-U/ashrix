package tunnel

type RequestEnvelope struct {
	AppID string `json:"app_id"`
	Method string `json:"method"`
	Path string `json:"path"`
	Query string `json:"query"`
	Headers map[string]string `json:"headers"`
	BodyLen int64 `json:"body_len"`
	UserID string `json:"user_id"`
	UserEmail string `json:"user_email"`
	RequestID string `json:"request_id"`
}

type ResponseEnvelope struct {
	StatusCode int `json:"status_code"`
	Headers map[string]string `json:"headers"`
}
