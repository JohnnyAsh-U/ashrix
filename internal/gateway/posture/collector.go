package posture

import (
	"net/http"
	"time"
)

// ============================================================
// 3. COLLECTOR (internal/posture/collector.go)
// ============================================================
// The Collector gathers all passive signals from an HTTP request.
// It is the single place where posture data is assembled.
// ============================================================

type Collector struct {
	geoReader    GeoIPReader      // MaxMind GeoIP2 reader
	torChecker   TorChecker       // Tor exit node checker
	reputationDB IPReputationDB   // IP reputation database (optional)
}

// NewCollector creates a posture collector with the given dependencies.
func NewCollector(geo GeoIPReader, tor TorChecker, rep IPReputationDB) *Collector {
	return &Collector{
		geoReader:    geo,
		torChecker:   tor,
		reputationDB: rep,
	}
}

// Collect gathers all posture signals from an HTTP request.
// This is called once per request by the posture middleware.
func (c *Collector) Collect(r *http.Request) (*DevicePosture, error) {
	now := time.Now()

	// 1. Extract client IP (X-Forwarded-For aware)
	clientIP := extractClientIP(r)

	// 2. Parse User-Agent
	ua := parseUserAgent(r.UserAgent())

	// 3. Geo lookup
	geo := c.geoReader.Lookup(clientIP)

	// 4. Tor check
	isTor := c.torChecker.IsTorExitNode(clientIP)

	// 5. IP reputation (optional, can be slow)
	reputation := "unknown"
	if c.reputationDB != nil {
		reputation = c.reputationDB.Check(clientIP)
	}

	dp := &DevicePosture{
		UserAgent:      r.UserAgent(),
		SourceIP:       clientIP,
		GeoCountry:     geo.Country,
		GeoCity:        geo.City,
		GeoASN:         geo.ASN,
		IsTorExit:      isTor,
		IPReputation:   reputation,
		OS:             ua.OS,
		OSVersion:      ua.OSVersion,
		Browser:        ua.Browser,
		BrowserVersion: ua.BrowserVersion,
		Status:         "passive", // MVP: no agent, so status is always "passive"
		CollectedAt:    now,
	}

	return dp, nil
}
