package http_proxy

import (
	"net/http"
	"strings"
)

func identityHeaders(in http.Header, userID, userEmail, appID, requestID string) http.Header {
	headers := in.Clone()
	for name := range headers {
		if strings.HasPrefix(strings.ToLower(name), "x-ashrix-") {
			delete(headers, name)
		}
	}
	if userID != "" {
		headers.Set("X-Ashrix-User-ID", userID)
	}
	if userEmail != "" {
		headers.Set("X-Ashrix-User-Email", userEmail)
	}
	if appID != "" {
		headers.Set("X-Ashrix-App-ID", appID)
	}
	if requestID != "" {
		headers.Set("X-Ashrix-Request-ID", requestID)
	}
	return headers
}
