package proxy

import "testing"

// func TestNormalizeHost(t *testing.T) {
// 	tests := []struct{ input, want string }{
// 		{"Example.COM", "example.com"},
// 		{"Example.COM.", "example.com"},
// 		{"Example.COM:8443", "example.com"},
// 		{"[2001:DB8::1]", "2001:db8::1"},
// 		{"[2001:DB8::1]:8443", "2001:db8::1"},
// 		{"localhost:80", "localhost"},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.input, func(t *testing.T) {
// 			if got := normalizeHost(tt.input); got != tt.want {
// 				t.Fatalf("normalizeHost(%q) = %q, want %q", tt.input, got, tt.want)
// 			}
// 		})
// 	}
// }

func TestIsInternalPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/_ashrix/health", true},
		{"/_ashrix/logout", true},
		{"/_ashrix/auth/callback", true},
		{"/_ashrix/static/app.js", true},
		{"/", false},
		{"/_ashrix/healthz", false},
		{"/_ashrix/auth", false},
		{"/_ashrix/static", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := isInternalPath(tt.path); got != tt.want {
				t.Fatalf("isInternalPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
