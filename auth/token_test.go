package auth

import (
	"testing"
	"time"
)

// Sample JWTs for testing. These are synthetic tokens with realistic structure
// but fake signatures. NOT real Steam tokens.
// Generated with valid base64url encoding for all three JWT segments.

// Access token: aud=["web:community","web:store"], exp=2000000000
const testAccessJWT = "eyJ0eXAiOiAiSldUIiwgImFsZyI6ICJFZERTQSJ9.eyJpc3MiOiAic3RlYW0iLCAic3ViIjogIjc2NTYxMTk4MDEyMzQ1Njc4IiwgImF1ZCI6IFsid2ViOmNvbW11bml0eSIsICJ3ZWI6c3RvcmUiXSwgImV4cCI6IDIwMDAwMDAwMDAsICJuYmYiOiAxNzAwMDAwMDAwLCAiaWF0IjogMTcwMDAwMDAwMCwgImp0aSI6ICJ0ZXN0MTIzIiwgInBlciI6IDB9.ZmFrZXNpZzEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2"

// Refresh token: aud=["derive"], exp=2000000000
const testRefreshJWT = "eyJ0eXAiOiAiSldUIiwgImFsZyI6ICJFZERTQSJ9.eyJpc3MiOiAic3RlYW0iLCAic3ViIjogIjc2NTYxMTk4MDEyMzQ1Njc4IiwgImF1ZCI6IFsiZGVyaXZlIl0sICJleHAiOiAyMDAwMDAwMDAwLCAibmJmIjogMTcwMDAwMDAwMCwgImlhdCI6IDE3MDAwMDAwMDAsICJqdGkiOiAicmVmcmVzaDEyMyIsICJwZXIiOiAwfQ.ZmFrZXNpZzEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2"

// Expired token: aud=["web:community"], exp=1000000000 (already past)
const testExpiredJWT = "eyJ0eXAiOiAiSldUIiwgImFsZyI6ICJFZERTQSJ9.eyJpc3MiOiAic3RlYW0iLCAic3ViIjogIjc2NTYxMTk4MDEyMzQ1Njc4IiwgImF1ZCI6IFsid2ViOmNvbW11bml0eSJdLCAiZXhwIjogMTAwMDAwMDAwMCwgIm5iZiI6IDkwMDAwMDAwMCwgImlhdCI6IDkwMDAwMDAwMCwgImp0aSI6ICJleHBpcmVkIiwgInBlciI6IDB9.ZmFrZXNpZzEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2"

func TestParseToken_AccessToken(t *testing.T) {
	token, err := ParseToken(testAccessJWT)
	if err != nil {
		t.Fatalf("ParseToken() error: %v", err)
	}

	if token.Claims.Issuer != "steam" {
		t.Errorf("issuer = %q, want %q", token.Claims.Issuer, "steam")
	}

	if token.Claims.Subject != "76561198012345678" {
		t.Errorf("subject = %q, want %q", token.Claims.Subject, "76561198012345678")
	}

	if !token.IsAccessToken() {
		t.Error("expected IsAccessToken() = true")
	}

	if token.IsRefreshToken() {
		t.Error("expected IsRefreshToken() = false")
	}

	if token.Platform() != PlatformWeb {
		t.Errorf("platform = %v, want Web", token.Platform())
	}

	if token.Expired() {
		t.Error("token should not be expired (exp=2000000000)")
	}

	steamID, err := token.SteamID64()
	if err != nil {
		t.Fatalf("SteamID64() error: %v", err)
	}
	if steamID != 76561198012345678 {
		t.Errorf("SteamID64() = %d, want 76561198012345678", steamID)
	}

	// Verify expiry time
	expectedExpiry := time.Unix(2000000000, 0)
	if !token.ExpiresAt().Equal(expectedExpiry) {
		t.Errorf("ExpiresAt() = %v, want %v", token.ExpiresAt(), expectedExpiry)
	}
}

func TestParseToken_RefreshToken(t *testing.T) {
	token, err := ParseToken(testRefreshJWT)
	if err != nil {
		t.Fatalf("ParseToken() error: %v", err)
	}

	if !token.IsRefreshToken() {
		t.Error("expected IsRefreshToken() = true")
	}

	if token.IsAccessToken() {
		t.Error("expected IsAccessToken() = false")
	}

	// "derive" audience means refresh token
	found := false
	for _, aud := range token.Claims.Audience {
		if aud == "derive" {
			found = true
		}
	}
	if !found {
		t.Error("refresh token should have 'derive' in audience")
	}
}

func TestParseToken_ExpiredToken(t *testing.T) {
	token, err := ParseToken(testExpiredJWT)
	if err != nil {
		t.Fatalf("ParseToken() error: %v", err)
	}

	if !token.Expired() {
		t.Error("expected Expired() = true for token with exp=1000000000")
	}
}

func TestParseToken_CookieValue(t *testing.T) {
	token, err := ParseToken(testAccessJWT)
	if err != nil {
		t.Fatalf("ParseToken() error: %v", err)
	}

	cv := token.CookieValue()
	expected := "76561198012345678" + CookieSeparator + testAccessJWT
	if cv != expected {
		t.Errorf("CookieValue() = %q, want %q", cv, expected)
	}

	// Round-trip: parse from cookie value
	parsed, err := ParseTokenFromCookie(cv)
	if err != nil {
		t.Fatalf("ParseTokenFromCookie() error: %v", err)
	}
	if parsed.Raw != token.Raw {
		t.Errorf("round-trip failed: got different raw token")
	}
}

func TestParseToken_InvalidFormats(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"no_dots", "nodots"},
		{"one_dot", "one.dot"},
		{"too_many_dots", "a.b.c.d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseToken(tt.input)
			if err == nil {
				t.Error("expected error for invalid JWT format")
			}
		})
	}
}

func TestParseTokenFromCookie_InvalidFormat(t *testing.T) {
	_, err := ParseTokenFromCookie("noseparator")
	if err == nil {
		t.Error("expected error for missing separator")
	}
}

func TestSteamToken_String(t *testing.T) {
	token, _ := ParseToken(testAccessJWT)
	s := token.String()

	if s == "" {
		t.Error("String() should not be empty")
	}

	// Should NOT contain the raw token (security)
	if len(s) > 200 {
		t.Error("String() seems to contain the raw token — it should be a summary only")
	}

	// Should contain type info
	if !contains(s, "Access") {
		t.Errorf("String() = %q, should contain 'Access'", s)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
