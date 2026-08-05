package policy

import (
	"fmt"
	"net"
	"time"

	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

type CompiledPolicy struct {
	Proto    *proto.PolicyRule
	Effect   Effect
	PolicyID string
	Priority int
	Version  int64

	AppIDs         map[string]struct{}
	Groups         map[string]struct{}
	Users          map[string]struct{}
	Methods        map[string]struct{}
	Countries      map[string]struct{}
	BlockCountries map[string]struct{}
	AllowedCIDRs   []*net.IPNet
	BlockedCIDRs   []*net.IPNet
	PathMatchers   []pathMatcher

	BlockTor       bool
	RequireMFA     bool
	MinMFALevel    string
	RequirePosture []string
	HasTimeCond    bool
	ScheduleName   string
}

func (cp *CompiledPolicy) matchesSubject(p Principal) bool {
	if len(cp.Groups) > 0 {
		matched := false
		for _, g := range p.Groups {
			if _, ok := cp.Groups[g]; ok {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if len(cp.Users) > 0 {
		if _, ok := cp.Users[p.UserID]; !ok {
			return false
		}
	}
	return true
}

func (cp *CompiledPolicy) matchesResource(r Resource) bool {
	if len(cp.AppIDs) > 0 {
		if _, ok := cp.AppIDs[r.AppID]; !ok {
			if _, ok := cp.AppIDs["*"]; !ok {
				return false
			}
		}
	}
	if len(cp.Methods) > 0 {
		if _, ok := cp.Methods[r.Method]; !ok {
			if _, ok := cp.Methods["*"]; !ok {
				return false
			}
		}
	}

	if len(cp.PathMatchers) > 0 {
		matched := false
		for _, pm := range cp.PathMatchers {
			if pm.match(r.Path) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func (cp *CompiledPolicy) matchesConditions(ctx AuthorizationContext, schedules map[string]*proto.Schedule) (bool, string) {
	if cp.RequireMFA {
		if ctx.Principal.MFALevel == "" {
			return false, "mfa_required"
		}
		if cp.MinMFALevel != "" {
			if !mfaLevelSufficient(ctx.Principal.MFALevel, cp.MinMFALevel) {
				return false, fmt.Sprintf("mfa_level_insufficient: got %s, need %s", ctx.Principal.MFALevel, cp.MinMFALevel)
			}
		}
	}

	if len(cp.RequirePosture) > 0 {
		matched := false
		for _, p := range cp.RequirePosture {
			if ctx.Device.Posture == p {
				matched = true
				break
			}
		}
		if !matched {
			return false, fmt.Sprintf("device_posture_failed: got %s, need %v", ctx.Device.Posture, cp.RequirePosture)
		}
	}

	if cp.BlockTor && ctx.Network.IsTorExit {
		return false, "tor_exit_blocked"
	}

	if len(cp.Countries) > 0 {
		if _, ok := cp.Countries[ctx.Network.Country]; !ok {
			return false, fmt.Sprintf("country_not_allowed: %s", ctx.Network.Country)
		}
	}

	if len(cp.BlockCountries) > 0 {
		if _, ok := cp.BlockCountries[ctx.Network.Country]; ok {
			return false, fmt.Sprintf("country_blocked: %s", ctx.Network.Country)
		}
	}

	if len(cp.AllowedCIDRs) > 0 {
		allowed := false
		for _, cidr := range cp.AllowedCIDRs {
			if cidr.Contains(ctx.Network.SourceIP) {
				allowed = true
				break
			}
		}
		if !allowed {
			return false, fmt.Sprintf("cidr_not_allowed: %s", ctx.Network.SourceIP)
		}
	}

	if len(cp.BlockedCIDRs) > 0 {
		for _, cidr := range cp.BlockedCIDRs {
			if cidr.Contains(ctx.Network.SourceIP) {
				return false, fmt.Sprintf("cidr_blocked: %s", ctx.Network.SourceIP)
			}
		}
	}

	// Time condition: fail-closed if schedule is missing or outside window.
	if cp.HasTimeCond {
		sched, ok := schedules[cp.ScheduleName]
		if !ok || sched == nil {
			return false, "schedule_not_found"
		}
		if !isScheduleActive(sched, ctx.Time) {
			return false, "outside_schedule"
		}
	}

	return true, ""
}

func mfaLevelSufficient(have, need string) bool {
	levels := map[string]int{"": 0, "totp": 1, "push": 2, "webauthn": 3}
	return levels[have] >= levels[need]
}

// isScheduleActive evaluates whether t falls within the schedule.
// TODO: Adapt field accessors to your actual proto.Schedule definition.
func isScheduleActive(sched *proto.Schedule, t time.Time) bool {
	// Placeholder: replace with actual day-of-week / time-range logic
	// based on your proto.Schedule fields. Returning true here means
	// the schedule is not enforced until you wire your protobuf.
	_ = sched
	_ = t
	return true
}