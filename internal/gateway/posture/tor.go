package posture

import (
	"slices"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// ============================================================
// 7. TOR CHECKER (internal/posture/tor.go)
// ============================================================
// Checks if an IP is a known Tor exit node.
// Uses the Tor Project's exit list or a local cache.
// ============================================================

type TorChecker interface {
	IsTorExitNode(ip net.IP) bool
}

// DNSBasedTorChecker queries the Tor Project DNS exit list.
// Format: <reverse_ip>.<port>.<list_name>.ip-port.exitlist.torproject.org
// If the query resolves to 127.0.0.2, the IP is a Tor exit node.
type DNSBasedTorChecker struct {
	listName string // e.g., "dnsel"
	port     string // e.g., "443"
}

func NewDNSBasedTorChecker() *DNSBasedTorChecker {
	return &DNSBasedTorChecker{
		listName: "dnsel",
		port:     "443", // Check if IP exits to port 443 (HTTPS)
	}
}

func (d *DNSBasedTorChecker) IsTorExitNode(ip net.IP) bool {
	if ip == nil || ip.To4() == nil {
		return false // Only IPv4 supported by this method
	}

	// Reverse the IP: 1.2.3.4 → 4.3.2.1
	octets := strings.Split(ip.String(), ".")
	reverse := fmt.Sprintf("%s.%s.%s.%s", octets[3], octets[2], octets[1], octets[0])

	query := fmt.Sprintf("%s.%s.%s.ip-port.exitlist.torproject.org",
		reverse, d.port, d.listName)

	// DNS lookup
	addrs, err := net.LookupHost(query)
	if err != nil {
		return false
	}

	return slices.Contains(addrs, "127.0.0.2")
}

// CachedTorChecker wraps another checker with an in-memory cache.
// Tor exit nodes change slowly, so caching for 1 hour is safe.
type CachedTorChecker struct {
	checker TorChecker
	cache   map[string]cachedTorResult
	mu      sync.RWMutex
	ttl     time.Duration
}

type cachedTorResult struct {
	isTor     bool
	cachedAt  time.Time
}

func NewCachedTorChecker(checker TorChecker, ttl time.Duration) *CachedTorChecker {
	return &CachedTorChecker{
		checker: checker,
		cache:   make(map[string]cachedTorResult),
		ttl:     ttl,
	}
}

func (c *CachedTorChecker) IsTorExitNode(ip net.IP) bool {
	if ip == nil {
		return false
	}
	key := ip.String()

	c.mu.RLock()
	cached, ok := c.cache[key]
	c.mu.RUnlock()

	if ok && time.Since(cached.cachedAt) < c.ttl {
		return cached.isTor
	}

	isTor := c.checker.IsTorExitNode(ip)

	c.mu.Lock()
	c.cache[key] = cachedTorResult{isTor: isTor, cachedAt: time.Now()}
	c.mu.Unlock()

	return isTor
}

// NoOpTorChecker always returns false.
type NoOpTorChecker struct{}

func (n *NoOpTorChecker) IsTorExitNode(ip net.IP) bool { return false }
