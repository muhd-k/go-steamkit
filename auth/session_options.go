package auth

import "net/http"

// Platform identifies the platform type for a Steam session.
type Platform int

const (
	// PlatformWeb creates a standard web browser session.
	// Access tokens are scoped to web domains (store, community, etc.).
	PlatformWeb Platform = iota

	// PlatformMobile creates a Steam Mobile App session.
	// Required for mobile confirmations (trade confirms, market listings, etc.).
	PlatformMobile
)

// String returns a human-readable platform name.
func (p Platform) String() string {
	switch p {
	case PlatformWeb:
		return "Web"
	case PlatformMobile:
		return "Mobile"
	default:
		return "Unknown"
	}
}

// SessionOption configures a SteamSession during construction.
// Use the With* functions to create option values.
type SessionOption func(*sessionConfig)

type sessionConfig struct {
	platform          Platform
	httpClient        *http.Client
	accessToken       string
	refreshToken      string
	deviceFriendlyName string
}

func defaultConfig() *sessionConfig {
	return &sessionConfig{
		platform:          PlatformWeb,
		deviceFriendlyName: "go-steamkit",
	}
}

// WithPlatform sets the platform type for the session.
//
// Use PlatformMobile if you need to generate or approve mobile confirmations
// (required for automated trade offer confirmation).
//
// Default: PlatformWeb
func WithPlatform(p Platform) SessionOption {
	return func(c *sessionConfig) {
		c.platform = p
	}
}

// WithHTTPClient provides a custom *http.Client for all HTTP requests.
//
// Use this to configure proxies, custom TLS settings, or timeouts.
// The client MUST have a CookieJar set — if it doesn't, one will be created.
//
// Default: a new http.Client with a standard cookie jar.
func WithHTTPClient(client *http.Client) SessionOption {
	return func(c *sessionConfig) {
		c.httpClient = client
	}
}

// WithAccessToken provides a previously obtained access token.
//
// Used to restore a session without re-authenticating.
// The token will be parsed and validated (expiry checked).
// Should be used together with WithRefreshToken for a complete session.
func WithAccessToken(token string) SessionOption {
	return func(c *sessionConfig) {
		c.accessToken = token
	}
}

// WithRefreshToken provides a previously obtained refresh token.
//
// Used to restore a session without re-authenticating.
// A new access token can be generated from the refresh token via RefreshAccessToken.
func WithRefreshToken(token string) SessionOption {
	return func(c *sessionConfig) {
		c.refreshToken = token
	}
}

// WithDeviceFriendlyName sets the device name used during authentication.
// This appears in the Steam account's "Authorized Devices" list.
//
// Default: "go-steamkit"
func WithDeviceFriendlyName(name string) SessionOption {
	return func(c *sessionConfig) {
		c.deviceFriendlyName = name
	}
}
