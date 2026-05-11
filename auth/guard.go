package auth

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

// TOTP code alphabet used by Steam Guard.
// This is NOT standard TOTP — Steam uses a custom 26-character alphabet
// instead of the standard base32/numeric codes.
const guardAlphabet = "23456789BCDFGHJKMNPQRTVWXY"

// ErrInvalidSharedSecret is returned when the shared secret is not valid base64.
var ErrInvalidSharedSecret = errors.New("steamkit/auth: invalid base64 shared secret")

// GenerateGuardCode generates a 5-character Steam Guard TOTP code from a shared secret.
//
// The sharedSecret must be the base64-encoded shared_secret from your Steam maFile.
// The code is time-sensitive and valid for approximately 30 seconds.
// For best results, generate the code immediately before submitting it.
//
// This implements Steam's custom TOTP algorithm which uses:
//   - HMAC-SHA1 (same as standard TOTP)
//   - 30-second time step (same as standard TOTP)
//   - Custom 26-character alphabet instead of numeric digits
//   - 5-character output instead of 6 digits
//
// Example:
//
//	code, err := auth.GenerateGuardCode("base64SharedSecret==")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println("Guard code:", code) // e.g. "R8FKV"
func GenerateGuardCode(sharedSecret string) (string, error) {
	return GenerateGuardCodeAtTime(sharedSecret, time.Now())
}

// GenerateGuardCodeAtTime generates a Steam Guard TOTP code for a specific time.
// This is primarily useful for testing with known time values.
func GenerateGuardCodeAtTime(sharedSecret string, t time.Time) (string, error) {
	key, err := base64.StdEncoding.DecodeString(sharedSecret)
	if err != nil {
		return "", ErrInvalidSharedSecret
	}

	return generateCode(key, t), nil
}

// generateCode performs the actual TOTP calculation.
func generateCode(key []byte, t time.Time) string {
	// Time step: divide Unix timestamp by 30-second intervals
	timeStep := uint64(t.Unix()) / 30

	// Encode time step as big-endian 8-byte value
	msg := make([]byte, 8)
	binary.BigEndian.PutUint64(msg, timeStep)

	// HMAC-SHA1
	mac := hmac.New(sha1.New, key)
	mac.Write(msg)
	hash := mac.Sum(nil)

	// Dynamic truncation: use last 4 bits of hash as offset
	offset := hash[19] & 0x0F

	// Extract 4 bytes at offset, mask off the sign bit
	code := binary.BigEndian.Uint32(hash[offset:offset+4]) & 0x7FFFFFFF

	// Convert to 5-character code using Steam's alphabet
	result := make([]byte, 5)
	for i := range result {
		result[i] = guardAlphabet[code%uint32(len(guardAlphabet))]
		code /= uint32(len(guardAlphabet))
	}

	return string(result)
}

// Signer wraps a shared secret for repeated Steam Guard code generation.
// It is safe for concurrent use.
//
// Use this when you need to generate multiple codes over time (e.g., in a long-running bot).
// The shared secret is decoded once at construction time.
//
// Example:
//
//	signer, err := auth.NewSigner("base64SharedSecret==")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	code, err := signer.Code()
//	fmt.Println("Guard code:", code)
type Signer struct {
	key []byte
}

// NewSigner creates a new Signer from a base64-encoded shared secret.
// The shared secret is validated and decoded at construction time.
func NewSigner(sharedSecret string) (*Signer, error) {
	key, err := base64.StdEncoding.DecodeString(sharedSecret)
	if err != nil {
		return nil, ErrInvalidSharedSecret
	}
	if len(key) == 0 {
		return nil, fmt.Errorf("steamkit/auth: shared secret decoded to empty bytes")
	}
	return &Signer{key: key}, nil
}

// Code generates a fresh Steam Guard TOTP code using the current time.
// Each call produces a code valid for ~30 seconds.
func (s *Signer) Code() string {
	return generateCode(s.key, time.Now())
}

// CodeAt generates a Steam Guard TOTP code for a specific time.
func (s *Signer) CodeAt(t time.Time) string {
	return generateCode(s.key, t)
}
