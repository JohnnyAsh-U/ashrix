package posture

import (
	"fmt"
	"net"
)

// ============================================================
// 8. IP REPUTATION (internal/posture/reputation.go)
// ============================================================
// Optional: checks IP reputation against external databases.
// Can be slow, so cache aggressively or skip in MVP.
// ============================================================

type IPReputationDB interface {
	Check(ip net.IP) string // returns "clean", "suspicious", or "malicious"
}

// StaticReputationDB uses a local list of known bad IPs/CIDRs.
// Fast but requires manual maintenance.
type StaticReputationDB struct {
	blockedCIDRs []*net.IPNet
}

func NewStaticReputationDB(cidrList []string) (*StaticReputationDB, error) {
	db := &StaticReputationDB{}
	for _, cidr := range cidrList {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %q: %w", cidr, err)
		}
		db.blockedCIDRs = append(db.blockedCIDRs, ipnet)
	}
	return db, nil
}

func (s *StaticReputationDB) Check(ip net.IP) string {
	for _, cidr := range s.blockedCIDRs {
		if cidr.Contains(ip) {
			return "malicious"
		}
	}
	return "clean"
}