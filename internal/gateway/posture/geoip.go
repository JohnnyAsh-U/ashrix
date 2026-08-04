package posture

import (
	"fmt"
	"net"
	"github.com/oschwald/geoip2-golang"
)

// ============================================================
// 6. GEOIP (internal/posture/geoip.go)
// ============================================================
// MaxMind GeoIP2 integration for country/city/ASN lookup.
// Requires downloading the GeoLite2-City.mmdb database.
// ============================================================

type GeoIPReader interface {
	Lookup(ip net.IP) GeoResult
}

type GeoResult struct {
	Country string
	City    string
	ASN     uint32
}

// MaxMindReader wraps the official MaxMind GeoIP2 reader.
type MaxMindReader struct {
	reader *geoip2.Reader // github.com/oschwald/geoip2-golang
}

func NewMaxMindReader(dbPath string) (*MaxMindReader, error) {
	reader, err := geoip2.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open geoip db: %w", err)
	}
	return &MaxMindReader{reader: reader}, nil
}

func (m *MaxMindReader) Lookup(ip net.IP) GeoResult {
	result := GeoResult{}
	if ip == nil || ip.IsPrivate() || ip.IsLoopback() {
		return result
	}

	// Country lookup
	country, err := m.reader.Country(ip)
	if err == nil && country.Country.IsoCode != "" {
		result.Country = country.Country.IsoCode
	}

	// City lookup
	city, err := m.reader.City(ip)
	if err == nil && len(city.City.Names) > 0 {
		result.City = city.City.Names["en"]
	}

	// ASN lookup (requires GeoLite2-ASN.mmdb)
	// asn, err := m.reader.ASN(ip)
	// if err == nil {
	//     result.ASN = asn.AutonomousSystemNumber
	// }

	return result
}

// NoOpReader returns empty results (for testing or when GeoIP is disabled).
type NoOpReader struct{}

func (n *NoOpReader) Lookup(ip net.IP) GeoResult { return GeoResult{} }
