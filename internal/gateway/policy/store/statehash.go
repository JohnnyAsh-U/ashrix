package store

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"

	pb "github.com/JohnnyAsh-U/ashrix-api/proto/gen"
	"google.golang.org/protobuf/proto"
)

// CalculateStateHash produces a deterministic SHA-256 fingerprint of the
// entire policy state. It binds to gatewayID + tenantID so an attacker
// cannot copy one tenant's database to another gateway.
//
// Determinism rules:
//   1. Sort rules by PolicyId (lexicographic).
//   2. Serialize each rule with proto.Marshal (binary, deterministic).
//   3. Concatenate and hash.
func CalculateStateHash(gatewayID, tenantID string, rules []*pb.PolicyRule) string {
	// Defensive copy so we don't mutate caller's slice
	sorted := make([]*pb.PolicyRule, len(rules))
	copy(sorted, rules)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].PolicyId < sorted[j].PolicyId
	})

	h := sha256.New()
	h.Write([]byte(gatewayID))
	h.Write([]byte{0x00})
	h.Write([]byte(tenantID))
	h.Write([]byte{0x00})

	for _, r := range sorted {
		b, err := proto.Marshal(r)
		if err != nil {
			// proto.Marshal on a valid message never errors
			panic(err)
		}
		h.Write(b)
	}

	return hex.EncodeToString(h.Sum(nil))
}