package posture

import "strings"

// ============================================================
// 5. USER-AGENT PARSING (internal/posture/browser.go)
// ============================================================
// Parses the User-Agent string to extract OS and browser info.
// This is MVP-level parsing. For production, use
// github.com/mileusna/useragent or github.com/avct/uasurfer.
// ============================================================

type UAParseResult struct {
	OS             string
	OSVersion      string
	Browser        string
	BrowserVersion string
}

func parseUserAgent(ua string) UAParseResult {
	ua = strings.ToLower(ua)
	result := UAParseResult{}

	// OS detection
	switch {
	case strings.Contains(ua, "macintosh") || strings.Contains(ua, "mac os"):
		result.OS = "macos"
		result.OSVersion = extractVersion(ua, "mac os x ")
	case strings.Contains(ua, "windows"):
		result.OS = "windows"
		result.OSVersion = extractVersion(ua, "windows ")
	case strings.Contains(ua, "linux") && !strings.Contains(ua, "android"):
		result.OS = "linux"
	case strings.Contains(ua, "android"):
		result.OS = "android"
		result.OSVersion = extractVersion(ua, "android ")
	case strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad"):
		result.OS = "ios"
		result.OSVersion = extractVersion(ua, "os ")
	default:
		result.OS = "unknown"
	}

	// Browser detection
	switch {
	case strings.Contains(ua, "edg/"):
		result.Browser = "edge"
		result.BrowserVersion = extractVersion(ua, "edg/")
	case strings.Contains(ua, "chrome") && !strings.Contains(ua, "edg/"):
		result.Browser = "chrome"
		result.BrowserVersion = extractVersion(ua, "chrome/")
	case strings.Contains(ua, "safari") && !strings.Contains(ua, "chrome"):
		result.Browser = "safari"
		result.BrowserVersion = extractVersion(ua, "version/")
	case strings.Contains(ua, "firefox"):
		result.Browser = "firefox"
		result.BrowserVersion = extractVersion(ua, "firefox/")
	default:
		result.Browser = "unknown"
	}

	return result
}

// extractVersion pulls a version number after a prefix.
// "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) ..."
// extractVersion(ua, "mac os x ") → "10_15_7"
func extractVersion(ua, prefix string) string {
	idx := strings.Index(ua, prefix)
	if idx == -1 {
		return ""
	}
	start := idx + len(prefix)
	end := start
	for end < len(ua) && (ua[end] == '.' || ua[end] == '_' || (ua[end] >= '0' && ua[end] <= '9')) {
		end++
	}
	return strings.ReplaceAll(ua[start:end], "_", ".")
}
