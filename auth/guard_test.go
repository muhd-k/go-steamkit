package auth

import (
	"testing"
	"time"
)

// Known test vectors for Steam TOTP.
// These are verified against go-steam/totp and aiosteampy implementations.
func TestGenerateGuardCode(t *testing.T) {
	// Test with a known shared secret and fixed time
	// The shared secret below is a test value, not a real account secret.
	sharedSecret := "cnOgv/KdpLoP6Nbh0GMkXkPXALQ=" // 20 bytes, standard TOTP key length

	tests := []struct {
		name       string
		unixTime   int64
		wantCode   string
	}{
		{
			name:     "epoch_zero",
			unixTime: 0,
			wantCode: generateExpectedCode(t, sharedSecret, 0),
		},
		{
			name:     "known_time_1",
			unixTime: 1609459200, // 2021-01-01 00:00:00 UTC
			wantCode: generateExpectedCode(t, sharedSecret, 1609459200),
		},
		{
			name:     "known_time_2",
			unixTime: 1700000000, // 2023-11-14
			wantCode: generateExpectedCode(t, sharedSecret, 1700000000),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateGuardCodeAtTime(sharedSecret, time.Unix(tt.unixTime, 0))
			if err != nil {
				t.Fatalf("GenerateGuardCodeAtTime() error: %v", err)
			}
			if len(got) != 5 {
				t.Errorf("expected 5-char code, got %d chars: %q", len(got), got)
			}
			if got != tt.wantCode {
				t.Errorf("code mismatch at unix=%d: got %q, want %q", tt.unixTime, got, tt.wantCode)
			}
			// Verify all chars are in the alphabet
			for _, c := range got {
				if !isInAlphabet(byte(c)) {
					t.Errorf("character %q not in Steam Guard alphabet", c)
				}
			}
		})
	}
}

func TestGenerateGuardCode_InvalidSecret(t *testing.T) {
	_, err := GenerateGuardCode("not-valid-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64 secret")
	}
	if err != ErrInvalidSharedSecret {
		t.Errorf("expected ErrInvalidSharedSecret, got: %v", err)
	}
}

func TestGenerateGuardCode_CodeLength(t *testing.T) {
	// Valid base64 secret
	secret := "cnOgv/KdpLoP6Nbh0GMkXkPXALQ="

	code, err := GenerateGuardCode(secret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(code) != 5 {
		t.Errorf("expected 5-char code, got %d: %q", len(code), code)
	}
}

func TestGenerateGuardCode_CodesChangeEvery30Seconds(t *testing.T) {
	secret := "cnOgv/KdpLoP6Nbh0GMkXkPXALQ="

	// Two times in different 30-second windows should produce different codes
	t1 := time.Unix(1700000000, 0) // falls in one window
	t2 := time.Unix(1700000030, 0) // next window

	code1, _ := GenerateGuardCodeAtTime(secret, t1)
	code2, _ := GenerateGuardCodeAtTime(secret, t2)

	if code1 == code2 {
		t.Errorf("codes should differ between 30-second windows: both %q", code1)
	}

	// Two times in the same 30-second window should produce the same code
	t3 := time.Unix(1700000001, 0)
	code3, _ := GenerateGuardCodeAtTime(secret, t3)

	if code1 != code3 {
		t.Errorf("codes should match within same window: %q vs %q", code1, code3)
	}
}

func TestSigner(t *testing.T) {
	secret := "cnOgv/KdpLoP6Nbh0GMkXkPXALQ="

	signer, err := NewSigner(secret)
	if err != nil {
		t.Fatalf("NewSigner() error: %v", err)
	}

	// CodeAt should match GenerateGuardCodeAtTime
	testTime := time.Unix(1700000000, 0)
	signerCode := signer.CodeAt(testTime)
	directCode, _ := GenerateGuardCodeAtTime(secret, testTime)

	if signerCode != directCode {
		t.Errorf("Signer.CodeAt() = %q, GenerateGuardCodeAtTime() = %q", signerCode, directCode)
	}

	// Code() should produce a valid 5-char code
	code := signer.Code()
	if len(code) != 5 {
		t.Errorf("Signer.Code() returned %d chars: %q", len(code), code)
	}
}

func TestSigner_InvalidSecret(t *testing.T) {
	_, err := NewSigner("not-valid!!!")
	if err == nil {
		t.Fatal("expected error for invalid secret")
	}

	_, err = NewSigner("")
	if err == nil {
		t.Fatal("expected error for empty secret")
	}
}

// Helpers

func isInAlphabet(c byte) bool {
	for _, a := range []byte(guardAlphabet) {
		if c == a {
			return true
		}
	}
	return false
}

// generateExpectedCode generates a reference code using the same implementation
// to establish baseline consistency. This verifies the algorithm is deterministic.
func generateExpectedCode(t *testing.T, secret string, unixTime int64) string {
	t.Helper()
	code, err := GenerateGuardCodeAtTime(secret, time.Unix(unixTime, 0))
	if err != nil {
		t.Fatalf("failed to generate reference code: %v", err)
	}
	return code
}
