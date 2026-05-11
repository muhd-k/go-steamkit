package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CookieSeparator is the separator used in Steam's steamLoginSecure cookie value
// between the SteamID64 and the JWT token.
const CookieSeparator = "%7C%7C"

// SteamToken represents a parsed Steam JWT token (either access or refresh).
//
// Steam issues two types of tokens during authentication:
//   - Access tokens: short-lived (~24h), used for API calls and web cookies
//   - Refresh tokens: long-lived (~200 days), used to obtain new access tokens
//
// Both are standard JWTs with Steam-specific claims in the payload.
type SteamToken struct {
	// Raw is the original encoded JWT string.
	Raw string

	// Header contains the JWT header (alg, typ).
	Header map[string]interface{}

	// Claims contains the decoded JWT payload with Steam-specific fields.
	Claims SteamTokenClaims

	// Signature is the raw JWT signature bytes.
	Signature []byte
}

// SteamTokenClaims contains the decoded claims from a Steam JWT.
type SteamTokenClaims struct {
	// Issuer — typically "steam".
	Issuer string `json:"iss"`

	// Subject — the SteamID64 as a string.
	Subject string `json:"sub"`

	// Audience — list of audience strings like "web:store", "web:community", "derive", "mobile".
	Audience []string `json:"aud"`

	// ExpiresAt — Unix timestamp when this token expires.
	ExpiresAt int64 `json:"exp"`

	// NotBefore — Unix timestamp before which the token is not valid.
	NotBefore int64 `json:"nbf"`

	// IssuedAt — Unix timestamp when the token was issued.
	IssuedAt int64 `json:"iat"`

	// JTI — unique token identifier.
	JTI string `json:"jti"`

	// OriginalAccessTokenIssuedAt — present in refresh tokens.
	OAT int64 `json:"oat,omitempty"`

	// RefreshTokenExpiresAt — present in access tokens, indicates when the
	// associated refresh token expires.
	RTExp int64 `json:"rt_exp,omitempty"`

	// Permissions bitmask.
	Per int `json:"per,omitempty"`

	// IPSubject — IP address used during login.
	IPSubject string `json:"ip_subject,omitempty"`

	// IPConfirmer — IP address that confirmed the login.
	IPConfirmer string `json:"ip_confirmer,omitempty"`
}

// ParseToken decodes a Steam JWT token string without verifying the signature.
// Steam tokens are self-contained — signature verification is not needed for
// client-side use since we trust the HTTPS connection to Steam.
//
// Returns an error if the token format is invalid or the payload cannot be decoded.
func ParseToken(encoded string) (*SteamToken, error) {
	parts := strings.SplitN(encoded, ".", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("steamkit/auth: invalid JWT format: expected 3 parts, got %d", len(parts))
	}

	headerBytes, err := base64URLDecode(parts[0])
	if err != nil {
		return nil, fmt.Errorf("steamkit/auth: failed to decode JWT header: %w", err)
	}

	claimsBytes, err := base64URLDecode(parts[1])
	if err != nil {
		return nil, fmt.Errorf("steamkit/auth: failed to decode JWT claims: %w", err)
	}

	sigBytes, err := base64URLDecode(parts[2])
	if err != nil {
		return nil, fmt.Errorf("steamkit/auth: failed to decode JWT signature: %w", err)
	}

	var header map[string]interface{}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("steamkit/auth: failed to parse JWT header: %w", err)
	}

	var claims SteamTokenClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return nil, fmt.Errorf("steamkit/auth: failed to parse JWT claims: %w", err)
	}

	return &SteamToken{
		Raw:       encoded,
		Header:    header,
		Claims:    claims,
		Signature: sigBytes,
	}, nil
}

// ParseTokenFromCookie parses a Steam JWT from a steamLoginSecure cookie value.
// The cookie format is: <steamid64>%7C%7C<jwt_token>
func ParseTokenFromCookie(cookieValue string) (*SteamToken, error) {
	parts := strings.SplitN(cookieValue, CookieSeparator, 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("steamkit/auth: invalid cookie value format: missing separator")
	}
	return ParseToken(parts[1])
}

// SteamID64 returns the SteamID64 from the token's subject claim as a uint64.
func (t *SteamToken) SteamID64() (uint64, error) {
	return strconv.ParseUint(t.Claims.Subject, 10, 64)
}

// ExpiresAt returns the expiration time of this token.
func (t *SteamToken) ExpiresAt() time.Time {
	return time.Unix(t.Claims.ExpiresAt, 0)
}

// IssuedAt returns the time this token was issued.
func (t *SteamToken) IssuedAt() time.Time {
	return time.Unix(t.Claims.IssuedAt, 0)
}

// Expired returns true if the token has expired.
func (t *SteamToken) Expired() bool {
	return time.Now().After(t.ExpiresAt())
}

// IsRefreshToken returns true if this is a refresh token.
// Refresh tokens have "derive" in their audience list.
func (t *SteamToken) IsRefreshToken() bool {
	for _, aud := range t.Claims.Audience {
		if aud == "derive" {
			return true
		}
	}
	return false
}

// IsAccessToken returns true if this is an access token (not a refresh token).
func (t *SteamToken) IsAccessToken() bool {
	return !t.IsRefreshToken()
}

// Platform returns the platform this token was issued for.
func (t *SteamToken) Platform() Platform {
	for _, aud := range t.Claims.Audience {
		if strings.HasPrefix(aud, "mobile") {
			return PlatformMobile
		}
		if aud == "client" {
			return PlatformWeb // treat client as web for our purposes
		}
	}
	return PlatformWeb
}

// CookieValue returns the token formatted as a Steam cookie value: <steamid64>%7C%7C<jwt>
func (t *SteamToken) CookieValue() string {
	return t.Claims.Subject + CookieSeparator + t.Raw
}

// String returns a human-readable summary of the token (does NOT expose the raw token).
func (t *SteamToken) String() string {
	kind := "Access"
	if t.IsRefreshToken() {
		kind = "Refresh"
	}
	return fmt.Sprintf("SteamToken(%s/%s, sub=%s, expires=%s)",
		kind, t.Platform(), t.Claims.Subject, t.ExpiresAt().Format(time.RFC3339))
}

// base64URLDecode decodes a base64url-encoded string with padding restoration.
func base64URLDecode(s string) ([]byte, error) {
	// Restore padding
	switch len(s) % 4 {
	case 2:
		s += "=="
	case 3:
		s += "="
	}
	return base64.URLEncoding.DecodeString(s)
}
