package proxy

import "testing"

func TestIsInternalPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/health", false},
		{"/_ashrix/health", true},
		{"/_ashrix/auth/callback", true},
		{"/auth/callback", false},
		{"/_ashrix/static/app.js", true},
		{"/static/app.js", false},
		{"/_ashrix/static", true},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := isInternalPath(tt.path); got != tt.want {
				t.Fatalf("isInternalPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
