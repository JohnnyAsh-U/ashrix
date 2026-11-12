package proxy

import (
	"net/http"
	"strings"
)

func identityHeaders(in http.Header, appID, requestID string) http.Header {
	headers := in.Clone()
	for name := range headers {
		if strings.HasPrefix(strings.ToLower(name), "x-ashrix-") {
			delete(headers, name)
		}
	}
	if appID != "" {
		headers.Set("X-Ashrix-App-ID", appID)
	}
	if requestID != "" {
		headers.Set("X-Ashrix-Request-ID", requestID)
	}
	return headers
}
