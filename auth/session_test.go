package auth

import (
	"errors"
	"testing"
)

func TestNewSession_Default(t *testing.T) {
	sess, err := NewSession()
	if err != nil {
		t.Fatalf("NewSession() error: %v", err)
	}

	if sess.Platform() != PlatformWeb {
		t.Errorf("default platform = %v, want Web", sess.Platform())
	}

	if sess.SteamID() != "" {
		t.Errorf("SteamID should be empty before login, got %q", sess.SteamID())
	}

	if sess.AccessToken() != nil {
		t.Error("AccessToken should be nil before login")
	}

	if sess.RefreshToken() != nil {
		t.Error("RefreshToken should be nil before login")
	}

	if sess.APIClient() == nil {
		t.Error("APIClient should not be nil")
	}
}

func TestNewSession_WithPlatform(t *testing.T) {
	sess, err := NewSession(WithPlatform(PlatformMobile))
	if err != nil {
		t.Fatalf("NewSession() error: %v", err)
	}

	if sess.Platform() != PlatformMobile {
		t.Errorf("platform = %v, want Mobile", sess.Platform())
	}
}

func TestNewSession_WithRefreshToken(t *testing.T) {
	sess, err := NewSession(WithRefreshToken(testRefreshJWT))
	if err != nil {
		t.Fatalf("NewSession() error: %v", err)
	}

	if sess.RefreshToken() == nil {
		t.Fatal("RefreshToken should not be nil")
	}

	if sess.SteamID() != "76561198012345678" {
		t.Errorf("SteamID = %q, want %q", sess.SteamID(), "76561198012345678")
	}
}

func TestNewSession_WithAccessToken(t *testing.T) {
	sess, err := NewSession(WithAccessToken(testAccessJWT))
	if err != nil {
		t.Fatalf("NewSession() error: %v", err)
	}

	if sess.AccessToken() == nil {
		t.Fatal("AccessToken should not be nil")
	}
}

func TestNewSession_WithExpiredAccessToken(t *testing.T) {
	_, err := NewSession(WithAccessToken(testExpiredJWT))
	if err == nil {
		t.Fatal("expected error for expired access token")
	}
	if !errors.Is(err, ErrTokenExpired) {
		t.Errorf("expected ErrTokenExpired, got: %v", err)
	}
}

func TestNewSession_WithBothTokens(t *testing.T) {
	sess, err := NewSession(
		WithRefreshToken(testRefreshJWT),
		WithAccessToken(testAccessJWT),
	)
	if err != nil {
		t.Fatalf("NewSession() error: %v", err)
	}

	if sess.AccessToken() == nil || sess.RefreshToken() == nil {
		t.Error("both tokens should be set")
	}
}

func TestSerializeDeserialize(t *testing.T) {
	sess, err := NewSession(
		WithRefreshToken(testRefreshJWT),
		WithAccessToken(testAccessJWT),
	)
	if err != nil {
		t.Fatalf("NewSession() error: %v", err)
	}

	// Serialize
	data := sess.Serialize()

	if data.SteamID != "76561198012345678" {
		t.Errorf("serialized SteamID = %q, want %q", data.SteamID, "76561198012345678")
	}

	if data.AccessToken != testAccessJWT {
		t.Error("serialized access token mismatch")
	}

	if data.RefreshToken != testRefreshJWT {
		t.Error("serialized refresh token mismatch")
	}

	// Deserialize
	restored, err := DeserializeSession(data)
	if err != nil {
		t.Fatalf("DeserializeSession() error: %v", err)
	}

	if restored.SteamID() != sess.SteamID() {
		t.Errorf("restored SteamID = %q, want %q", restored.SteamID(), sess.SteamID())
	}

	if restored.AccessToken().Raw != sess.AccessToken().Raw {
		t.Error("restored access token mismatch")
	}
}

func TestGuardRequiredError_Unwrap(t *testing.T) {
	err := &GuardRequiredError{
		DeviceCode:  true,
		AllowedTypes: []GuardType{GuardTypeDeviceCode},
	}

	if !errors.Is(err, ErrGuardRequired) {
		t.Error("GuardRequiredError should unwrap to ErrGuardRequired")
	}

	var guardErr *GuardRequiredError
	if !errors.As(err, &guardErr) {
		t.Error("errors.As should work with *GuardRequiredError")
	}

	if !guardErr.DeviceCode {
		t.Error("DeviceCode should be true")
	}
}

func TestGuardType_String(t *testing.T) {
	tests := []struct {
		gt   GuardType
		want string
	}{
		{GuardTypeNone, "None"},
		{GuardTypeDeviceCode, "DeviceCode"},
		{GuardTypeEmailCode, "EmailCode"},
		{GuardTypeDeviceConfirmation, "DeviceConfirmation"},
		{GuardTypeEmailConfirmation, "EmailConfirmation"},
	}

	for _, tt := range tests {
		if got := tt.gt.String(); got != tt.want {
			t.Errorf("GuardType(%d).String() = %q, want %q", tt.gt, got, tt.want)
		}
	}
}

func TestGenerateSessionID(t *testing.T) {
	id1 := generateSessionID()
	id2 := generateSessionID()

	if len(id1) != 24 {
		t.Errorf("sessionID length = %d, want 24", len(id1))
	}

	if id1 == id2 {
		t.Error("session IDs should be unique")
	}
}

func TestCookiesValid(t *testing.T) {
	sess, _ := NewSession()
	if sess.CookiesValid() {
		t.Error("CookiesValid should be false before login")
	}

	sess, _ = NewSession(WithAccessToken(testAccessJWT))
	if sess.CookiesValid() {
		t.Error("CookiesValid should be false without sessionID")
	}
}
