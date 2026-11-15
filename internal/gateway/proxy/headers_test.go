package proxy

import (
	"net/http"
	"testing"
)

func TestIdentityHeadersRemovesClientSuppliedAshrixHeaders(t *testing.T) {
	in := http.Header{
		"X-Ashrix-App-Id":     []string{"spoofed-app"},
		"x-ashrix-request-id": []string{"spoofed-request"},
		"X-Forwarded-For":     []string{"192.0.2.1"},
		"Content-Type":        []string{"application/json"},
	}

	got := identityHeaders(in, "trusted-app", "trusted-request")
	if got.Get("X-Ashrix-App-ID") != "trusted-app" {
		t.Fatalf("app identity = %q, want trusted-app", got.Get("X-Ashrix-App-ID"))
	}
	if got.Get("X-Ashrix-Request-ID") != "trusted-request" {
		t.Fatalf("request identity = %q, want trusted-request", got.Get("X-Ashrix-Request-ID"))
	}
	if got.Get("X-Forwarded-For") != "192.0.2.1" || got.Get("Content-Type") != "application/json" {
		t.Fatalf("ordinary headers were not preserved: %#v", got)
	}
	for name := range got {
		if len(name) >= len("X-Ashrix-") && equalFoldPrefix(name, "X-Ashrix-") {
			if name != "X-Ashrix-App-Id" && name != "X-Ashrix-Request-Id" {
				t.Errorf("unexpected client-supplied Ashrix header survived: %q", name)
			}
		}
	}
}

func TestIdentityHeadersDoesNotMutateInput(t *testing.T) {
	in := http.Header{"X-Ashrix-App-Id": []string{"attacker"}, "Accept": []string{"application/json"}}
	_ = identityHeaders(in, "app", "request")
	if in.Get("X-Ashrix-App-Id") != "attacker" {
		t.Fatal("identityHeaders mutated the caller's header map")
	}
}

func TestIdentityHeadersOmitsEmptyIdentityValues(t *testing.T) {
	got := identityHeaders(http.Header{"Accept": []string{"*/*"}}, "", "")
	if got.Get("X-Ashrix-App-ID") != "" || got.Get("X-Ashrix-Request-ID") != "" {
		t.Fatalf("unexpected identity headers: %#v", got)
	}
}

func equalFoldPrefix(value, prefix string) bool {
	if len(value) < len(prefix) {
		return false
	}
	for i := range prefix {
		a, b := value[i], prefix[i]
		if a >= 'A' && a <= 'Z' {
			a += 'a' - 'A'
		}
		if b >= 'A' && b <= 'Z' {
			b += 'a' - 'A'
		}
		if a != b {
			return false
		}
	}
	return true
}
