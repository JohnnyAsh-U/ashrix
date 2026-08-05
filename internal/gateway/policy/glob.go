package policy

import "strings"

// =============================================================================
// Path Matching (lightweight glob)
// =============================================================================

type pathMatcher struct {
	matchAll bool
	segments []string
}

func compilePathMatcher(pattern string) pathMatcher {
	pattern = strings.TrimSpace(pattern)
	if pattern == "*" || pattern == "/*" {
		return pathMatcher{matchAll: true}
	}
	pattern = strings.Trim(pattern, "/")
	if pattern == "" {
		return pathMatcher{segments: []string{""}}
	}
	return pathMatcher{segments: strings.Split(pattern, "/")}
}

func (pm pathMatcher) match(p string) bool {
	if pm.matchAll {
		return true
	}
	p = strings.Trim(p, "/")
	if p == "" {
		p = "/"
	}
	parts := strings.Split(p, "/")

	var match func(pi, pj int) bool
	match = func(pi, pj int) bool {
		if pi == len(pm.segments) && pj == len(parts) {
			return true
		}
		if pi >= len(pm.segments) {
			return false
		}

		switch pm.segments[pi] {
		case "**":
			// Match zero or more segments
			if pi == len(pm.segments)-1 {
				return true // trailing ** eats everything
			}
			for k := pj; k <= len(parts); k++ {
				if match(pi+1, k) {
					return true
				}
			}
			return false
		case "*":
			// Match exactly one segment
			if pj >= len(parts) {
				return false
			}
			return match(pi+1, pj+1)
		default:
			if pj >= len(parts) || pm.segments[pi] != parts[pj] {
				return false
			}
			return match(pi+1, pj+1)
		}
	}
	return match(0, 0)
}