package policy

import (
	"fmt"
	"net"

	proto "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
)

func Compile(rule *proto.PolicyRule) (*CompiledPolicy, error) {
	cp := &CompiledPolicy{
		Proto:    rule,
		Effect:   Effect(rule.Effect),
		PolicyID: rule.PolicyId,
		Priority: int(rule.Priority),
		Version:  rule.Version,
	}

	cp.Groups = makeSet(rule.Subject.Groups)
	cp.Users = makeSet(rule.Subject.Users)
	cp.AppIDs = makeSet(rule.Resource.AppIds)
	cp.Methods = makeSet(rule.Resource.Methods)
	if len(cp.Methods) == 0 {
		cp.Methods["*"] = struct{}{}
	}

	if rule.Conditions != nil {
		if rule.Conditions.Mfa != nil {
			cp.RequireMFA = rule.Conditions.Mfa.Required
			cp.MinMFALevel = rule.Conditions.Mfa.MinLevel
		}
		if rule.Conditions.Device != nil {
			cp.RequirePosture = rule.Conditions.Device.Postures
		}
		if rule.Conditions.Network != nil {
			nc := rule.Conditions.Network
			cp.BlockTor = nc.BlockTor
			cp.Countries = makeSet(nc.AllowedCountries)
			cp.BlockCountries = makeSet(nc.BlockedCountries)
			for _, cidr := range nc.AllowedCidrs {
				_, ipnet, err := net.ParseCIDR(cidr)
				if err != nil {
					return nil, fmt.Errorf("invalid allowed_cidr %q in policy %s: %w", cidr, cp.PolicyID, err)
				}
				cp.AllowedCIDRs = append(cp.AllowedCIDRs, ipnet)
			}
			for _, cidr := range nc.BlockedCidrs {
				_, ipnet, err := net.ParseCIDR(cidr)
				if err != nil {
					return nil, fmt.Errorf("invalid blocked_cidr %q in policy %s: %w", cidr, cp.PolicyID, err)
				}
				cp.BlockedCIDRs = append(cp.BlockedCIDRs, ipnet)
			}
		}
		if rule.Conditions.Time != nil {
			cp.HasTimeCond = true
			cp.ScheduleName = rule.Conditions.Time.ScheduleName
		}
	}

	return cp, nil
}

func makeSet(items []string) map[string]struct{} {
	m := make(map[string]struct{}, len(items))
	for _, item := range items {
		m[item] = struct{}{}
	}
	return m
}
