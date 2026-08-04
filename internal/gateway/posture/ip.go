package posture

import (
	"net"
	"net/http"
	"strings"
)

// ============================================================
// 4. CLIENT IP EXTRACTION (internal/posture/ip.go)
// ============================================================
// Extracts the real client IP from X-Forwarded-For, X-Real-IP,
// or the direct connection address. Handles trusted proxy chains.
// ============================================================

// TrustedProxies is a list of IP ranges that are allowed to set
// X-Forwarded-For. In production, this is your load balancer / CDN.
var TrustedProxies = []string{
	"10.0.0.0/8",      // AWS VPC internal
	"172.16.0.0/12",   // AWS VPC internal
	"192.168.0.0/16",  // AWS VPC internal
	"127.0.0.1/32",    // localhost
}

func extractClientIP(r *http.Request) net.IP {
	// Parse trusted proxy CIDRs once at startup
	// (omitted for brevity — use sync.Once in production)

	// 1. Check X-Forwarded-For (most common)
	// Format: "client, proxy1, proxy2" — leftmost is the original client
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// Take the leftmost IP that is NOT a trusted proxy
		ips := strings.Split(xff, ",")
		for _, ipStr := range ips {
			ipStr = strings.TrimSpace(ipStr)
			ip := net.ParseIP(ipStr)
			if ip == nil {
				continue
			}
			if !isTrustedProxy(ip) {
				return ip
			}
		}
	}

	// 2. Check X-Real-IP (used by some proxies)
	xri := r.Header.Get("X-Real-Ip")
	if xri != "" {
		if ip := net.ParseIP(xri); ip != nil && !isTrustedProxy(ip) {
			return ip
		}
	}

	// 3. Fall back to direct connection address
	// r.RemoteAddr is "ip:port"
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// RemoteAddr might not have a port (e.g., in tests)
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip
	}

	return nil
}

func isTrustedProxy(ip net.IP) bool {
	// In production, check against TrustedProxies CIDRs
	// For MVP, accept all private ranges
	return ip.IsPrivate() || ip.IsLoopback()
}