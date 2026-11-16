package proxy

import (
	"net/http"
	"testing"
)

func TestIdentityHeadersRemovesClientSuppliedAshrixHeaders(t *testing.T) {
	in := http.Header{
		"X-Ashrix-App-Id":     []string{"spoofed-app"},
		"x-ashrix-request-id": []string{"spoofed-request"},
	}

	got := identityHeaders(in, "trusted-app", "trusted-request")
	if got.Get("X-Ashrix-App-ID") != "trusted-app" {
		t.Fatalf("app identity = %q, want trusted-app", got.Get("X-Ashrix-App-ID"))
	}
	if got.Get("X-Ashrix-Request-ID") != "trusted-request" {
		t.Fatalf("request identity = %q, want trusted-request", got.Get("X-Ashrix-Request-ID"))
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
