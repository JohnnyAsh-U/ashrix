package crypto

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

// =============================================================================
// POSSESSION PROOF VERIFIER
// CP calls this before issuing any cert — renewal or recovery.
// =============================================================================

// VerifyPossessionProof checks that the requester holds the private key
// for their currently issued certificate.
//
//	proof_signature = SIGN(SHA256(csr_pem || node_id || timestamp_unix_big_endian), current_private_key)
func VerifyPossessionProof(
	currentCert *x509.Certificate,
	csrPEM []byte,
	nodeID string,
	timestamp time.Time,
	proofSignature []byte,
) error {
	// Reject stale proofs — prevents replay attacks
	age := time.Since(timestamp)
	if age > 5*time.Minute {
		return fmt.Errorf("possession proof too old: %v", age)
	}
	if timestamp.After(time.Now().Add(30 * time.Second)) {
		return errors.New("possession proof timestamp is in the future")
	}

	digest := possessionDigest(csrPEM, nodeID, timestamp)

	ecPub, ok := currentCert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return errors.New("current cert does not contain an EC public key")
	}
	if !ecdsa.VerifyASN1(ecPub, digest, proofSignature) {
		return errors.New("possession proof signature invalid")
	}
	return nil
}

// BuildPossessionProof is called by the node before sending IssueCert / RecoverCert.
// currentKey is the node's current private key — the one matching the current cert.
func BuildPossessionProof(
	csrPEM []byte,
	nodeID string,
	timestamp time.Time,
	currentKey *ecdsa.PrivateKey,
) ([]byte, error) {
	digest := possessionDigest(csrPEM, nodeID, timestamp)
	sig, err := ecdsa.SignASN1(rand.Reader, currentKey, digest)
	if err != nil {
		return nil, fmt.Errorf("signing possession proof: %w", err)
	}
	return sig, nil
}

// possessionDigest computes SHA256(csr_pem || node_id || timestamp_unix_big_endian).
// Must be identical on both the node (BuildPossessionProof) and CP (VerifyPossessionProof).
func possessionDigest(csrPEM []byte, nodeID string, timestamp time.Time) []byte {
	tsBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(tsBytes, uint64(timestamp.Unix()))

	combined := make([]byte, 0, len(csrPEM)+len(nodeID)+8)
	combined = append(combined, csrPEM...)
	combined = append(combined, []byte(nodeID)...)
	combined = append(combined, tsBytes...)

	digest := sha256.Sum256(combined)
	return digest[:]
}
