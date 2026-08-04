package posture


import (
	// "context"
	"fmt"
	"net"
	// "net/http"
	// "strings"
	"time"
)

// DevicePosture is the complete posture snapshot for a single request.
// It is built from passive signals (HTTP headers, connection metadata)
// and stored in context for downstream middleware to use.
type DevicePosture struct {
	// Identity
	UserAgent   string `json:"user_agent"`
	SourceIP    net.IP `json:"source_ip"`

	// Network context
	GeoCountry  string `json:"geo_country"`  // ISO 3166-1 alpha-2, e.g. "CI"
	GeoCity     string `json:"geo_city"`     // e.g. "Abidjan"
	GeoASN      uint32 `json:"geo_asn"`      // Autonomous system number
	IsTorExit   bool   `json:"is_tor_exit"`
	IPReputation string `json:"ip_reputation"` // "clean", "suspicious", "malicious"

	// Browser / OS fingerprint (from User-Agent)
	OS          string `json:"os"`          // "macos", "windows", "linux", "ios", "android"
	OSVersion   string `json:"os_version"`  // "14.5", "11", "22.04"
	Browser     string `json:"browser"`     // "chrome", "safari", "firefox", "edge"
	BrowserVersion string `json:"browser_version"` // "126.0.6478.127"

	// Posture status for policy engine
	// MVP: always "passive" since we have no agent
	// Post-MVP: "compliant", "non_compliant", "unknown"
	Status      string `json:"status"`      // "passive" for MVP
	DeviceID    string `json:"device_id"`   // empty until agent is deployed

	// Metadata
	CollectedAt time.Time `json:"collected_at"`
}

// IsOutdatedBrowser checks if the browser version is below a minimum.
// Used by the policy engine's MinBrowserVersion condition.
func (dp *DevicePosture) IsOutdatedBrowser(minVersions map[string]string) bool {
	minVer, ok := minVersions[dp.Browser]
	if !ok {
		return false // No minimum specified for this browser
	}
	// Simple string comparison works for major version checks
	// "126" >= "120" → true
	// For semantic versioning, use github.com/Masterminds/semver
	return dp.BrowserVersion < minVer
}

// String returns a human-readable posture summary for logs.
func (dp *DevicePosture) String() string {
	return fmt.Sprintf("posture[os=%s/%s browser=%s/%s country=%s tor=%v reputation=%s]",
		dp.OS, dp.OSVersion, dp.Browser, dp.BrowserVersion,
		dp.GeoCountry, dp.IsTorExit, dp.IPReputation)
}