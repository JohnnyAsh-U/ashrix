package proxy

import (
	"net/http"
	"testing"
)

func TestIsWebSocketRequestRequiresUpgradeAndConnectionTokens(t *testing.T) {
	tests := []struct {
		name   string
		header http.Header
		want   bool
	}{
		{"valid", http.Header{"Upgrade": []string{"websocket"}, "Connection": []string{"Upgrade"}}, true},
		{"case insensitive", http.Header{"Upgrade": []string{"WebSocket"}, "Connection": []string{"keep-alive, uPgRaDe"}}, true},
		{"missing connection", http.Header{"Upgrade": []string{"websocket"}}, false},
		{"missing upgrade", http.Header{"Connection": []string{"Upgrade"}}, false},
		{"wrong upgrade protocol", http.Header{"Upgrade": []string{"h2c"}, "Connection": []string{"Upgrade"}}, false},
		{"upgrade is only substring", http.Header{"Upgrade": []string{"websocket"}, "Connection": []string{"not-upgrade"}}, false},
		{"empty", make(http.Header), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &http.Request{Header: tt.header}
			if got := isWebSocketRequest(r); got != tt.want {
				t.Fatalf("isWebSocketRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}
