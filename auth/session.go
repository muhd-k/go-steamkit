package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/skinflippa/go-steamkit/internal/cryptoutil"
	"github.com/skinflippa/go-steamkit/internal/steamapi"
)

// SteamSession manages authentication state for a single Steam account.
//
// A session progresses through a lifecycle:
//
//  1. Created via [NewSession] — no authentication yet
//  2. Credentials submitted via [SteamSession.WithCredentials] or [SteamSession.LoginWithCredentials]
//  3. Guard code submitted (if required) via [SteamSession.SubmitAuthCode]
//  4. Finalized via [SteamSession.Finalize] — tokens are now available
//  5. Web cookies obtained via [SteamSession.ObtainCookies] — ready for web requests
//
// Alternatively, a session can be restored from saved tokens using [WithRefreshToken].
//
// The session is NOT safe for concurrent use. If you need concurrent access,
// protect it with a mutex or use separate sessions.
type SteamSession struct {
	// Configuration
	platform           Platform
	deviceFriendlyName string

	// API client
	api *steamapi.Client

	// Auth state (populated during login flow)
	clientID     uint64
	requestID    []byte // raw bytes from protobuf
	pollInterval float32
	steamID      uint64 // SteamID64

	// Tokens (populated after finalization)
	accessToken  *SteamToken
	refreshToken *SteamToken
	accountName  string

	// Web session
	sessionID string // random hex string used as sessionid cookie
}

// NewSession creates a new SteamSession with the given options.
//
// A freshly created session is not yet authenticated. Call WithCredentials or
// LoginWithCredentials to begin the login process, or provide pre-existing
// tokens via WithRefreshToken to restore a previous session.
//
// Example — fresh login:
//
//	sess, err := auth.NewSession(auth.WithPlatform(auth.PlatformMobile))
//
// Example — restore from saved token:
//
//	sess, err := auth.NewSession(auth.WithRefreshToken(savedToken))
func NewSession(opts ...SessionOption) (*SteamSession, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}

	apiClient := steamapi.NewClient(cfg.httpClient)

	sess := &SteamSession{
		platform:           cfg.platform,
		deviceFriendlyName: cfg.deviceFriendlyName,
		api:                apiClient,
	}

	// Restore tokens if provided
	if cfg.refreshToken != "" {
		token, err := ParseToken(cfg.refreshToken)
		if err != nil {
			return nil, fmt.Errorf("steamkit/auth: invalid refresh token: %w", err)
		}
		if !token.IsRefreshToken() {
			return nil, fmt.Errorf("steamkit/auth: provided token is not a refresh token")
		}
		sess.refreshToken = token
		steamID, _ := token.SteamID64()
		sess.steamID = steamID
		sess.platform = token.Platform()
	}

	if cfg.accessToken != "" {
		token, err := ParseToken(cfg.accessToken)
		if err != nil {
			return nil, fmt.Errorf("steamkit/auth: invalid access token: %w", err)
		}
		if !token.IsAccessToken() {
			return nil, fmt.Errorf("steamkit/auth: provided token is not an access token")
		}
		if token.Expired() {
			return nil, fmt.Errorf("steamkit/auth: provided access token has expired: %w", ErrTokenExpired)
		}
		sess.accessToken = token
		steamID, _ := token.SteamID64()
		sess.steamID = steamID
		sess.api.AccessToken = token.Raw
	}

	return sess, nil
}

// --- Properties ---

// Platform returns the platform type of this session.
func (s *SteamSession) Platform() Platform {
	return s.platform
}

// SteamID returns the SteamID64 as a string, or "" if not yet authenticated.
func (s *SteamSession) SteamID() string {
	if s.steamID == 0 {
		return ""
	}
	return strconv.FormatUint(s.steamID, 10)
}

// SteamID64 returns the SteamID64 as uint64, or 0 if not yet authenticated.
func (s *SteamSession) SteamID64() uint64 {
	return s.steamID
}

// AccountName returns the account name, or "" if not yet known.
func (s *SteamSession) AccountName() string {
	return s.accountName
}

// AccessToken returns the current access token, or nil if not available.
func (s *SteamSession) AccessToken() *SteamToken {
	return s.accessToken
}

// RefreshToken returns the current refresh token, or nil if not available.
func (s *SteamSession) RefreshToken() *SteamToken {
	return s.refreshToken
}

// SessionID returns the sessionid cookie value, or "" if cookies haven't been obtained.
func (s *SteamSession) SessionID() string {
	return s.sessionID
}

// APIClient returns the underlying Steam API client.
// Use this for direct API calls not covered by the session's methods.
func (s *SteamSession) APIClient() *steamapi.Client {
	return s.api
}

// --- Login Flow ---

// WithCredentials begins the authentication flow by submitting username and password.
//
// This method:
//  1. Fetches the RSA public key for the account
//  2. Encrypts the password
//  3. Calls IAuthenticationService/BeginAuthSessionViaCredentials
//
// If Steam requires a Guard confirmation, a *GuardRequiredError is returned.
// Use errors.As to extract the confirmation details, then call SubmitAuthCode.
//
// If no Guard is required (rare — accounts without Steam Guard), call Finalize directly.
//
// Returns ErrBadCredentials if the username/password is wrong.
// Returns ErrTooManyAttempts if rate-limited.
func (s *SteamSession) WithCredentials(ctx context.Context, accountName, password string) error {
	// Step 1: Get RSA public key
	rsaResp, err := s.api.GetPasswordRSAPublicKey(ctx, accountName)
	if err != nil {
		return fmt.Errorf("steamkit/auth: failed to get RSA public key: %w", err)
	}

	// Step 2: Encrypt password
	encryptedPassword, err := cryptoutil.EncryptPassword(
		password,
		rsaResp.PublicKeyMod,
		rsaResp.PublicKeyExp,
	)
	if err != nil {
		return fmt.Errorf("steamkit/auth: failed to encrypt password: %w", err)
	}

	// Step 3: Begin auth session
	beginResp, err := s.api.BeginAuthSessionViaCredentials(
		ctx,
		accountName,
		encryptedPassword,
		rsaResp.Timestamp,
		true, // persistence
		s.deviceFriendlyName,
	)
	if err != nil {
		// Check for known error patterns
		errStr := err.Error()
		if strings.Contains(errStr, "EResult 5") || strings.Contains(errStr, "InvalidPassword") {
			return ErrBadCredentials
		}
		if strings.Contains(errStr, "EResult 84") || strings.Contains(errStr, "RateLimitExceeded") {
			return ErrTooManyAttempts
		}
		return fmt.Errorf("steamkit/auth: begin auth session failed: %w", err)
	}

	// Store session state
	s.clientID = beginResp.ClientID
	s.requestID = beginResp.RequestID
	s.pollInterval = beginResp.Interval
	s.steamID = beginResp.SteamID
	s.accountName = accountName

	// Check allowed confirmations
	guardErr := parseAllowedConfirmations(beginResp.AllowedConfirmations)
	if guardErr != nil {
		return guardErr
	}

	return nil
}

// SubmitAuthCode submits a Steam Guard code for the current login session.
//
// guardType should be:
//   - GuardTypeDeviceCode for TOTP codes from the Steam Mobile Authenticator
//   - GuardTypeEmailCode for codes sent via email
//
// Call this after WithCredentials returns a *GuardRequiredError.
// After a successful submission, call Finalize to complete the login.
//
// Returns ErrAuthCodeInvalid if the code was wrong.
// Returns ErrAuthCodeExpired if the code has expired (generate a fresh one).
func (s *SteamSession) SubmitAuthCode(ctx context.Context, code string, guardType GuardType) error {
	if s.clientID == 0 || s.steamID == 0 {
		return ErrSessionNotReady
	}

	codeType := steamapi.GuardTypeDeviceCodeValue
	if guardType == GuardTypeEmailCode {
		codeType = steamapi.GuardTypeEmailCodeValue
	}

	err := s.api.UpdateAuthSessionWithSteamGuardCode(ctx, s.clientID, s.steamID, code, codeType)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "EResult 65") || strings.Contains(errStr, "EResult 88") {
			return ErrAuthCodeInvalid
		}
		if strings.Contains(errStr, "EResult 71") || strings.Contains(errStr, "EResult 27") {
			return ErrAuthCodeExpired
		}
		// Duplicate request is OK (code was already accepted)
		if strings.Contains(errStr, "EResult 29") {
			return nil
		}
		return fmt.Errorf("steamkit/auth: failed to submit guard code: %w", err)
	}

	return nil
}

// Finalize completes the login process by polling Steam for the issued tokens.
//
// Call this after:
//   - WithCredentials (if no Guard was required), or
//   - SubmitAuthCode (after Guard code was accepted)
//
// The timeout parameter controls how long to poll. If 0, a sensible default is
// calculated from Steam's suggested poll interval (~3 attempts).
//
// Returns the access and refresh tokens on success.
// Returns ErrFinalizeTimeout if polling exceeded the timeout.
func (s *SteamSession) Finalize(ctx context.Context, timeout time.Duration) (*SteamToken, *SteamToken, error) {
	if s.clientID == 0 || len(s.requestID) == 0 {
		return nil, nil, ErrSessionNotReady
	}

	if timeout == 0 {
		// Default: ~3 poll attempts with padding
		interval := float64(s.pollInterval)
		if interval < 1 {
			interval = 1
		}
		timeout = time.Duration(interval*3+1) * time.Second
	}

	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		pollResp, err := s.api.PollAuthSessionStatus(ctx, s.clientID, s.requestID)
		if err != nil {
			return nil, nil, fmt.Errorf("steamkit/auth: poll failed: %w", err)
		}

		if pollResp.RefreshToken != "" {
			// Success — parse and store tokens
			refreshToken, err := ParseToken(pollResp.RefreshToken)
			if err != nil {
				return nil, nil, fmt.Errorf("steamkit/auth: failed to parse refresh token: %w", err)
			}

			accessToken, err := ParseToken(pollResp.AccessToken)
			if err != nil {
				return nil, nil, fmt.Errorf("steamkit/auth: failed to parse access token: %w", err)
			}

			s.refreshToken = refreshToken
			s.accessToken = accessToken
			s.api.AccessToken = accessToken.Raw

			if pollResp.AccountName != "" {
				s.accountName = pollResp.AccountName
			}

			// Clear transient auth state
			s.clientID = 0
			s.requestID = nil
			s.pollInterval = 0

			return accessToken, refreshToken, nil
		}

		// Wait before next poll
		interval := float64(s.pollInterval)
		if interval < 0.5 {
			interval = 1
		}
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(time.Duration(interval * float64(time.Second))):
		}
	}

	return nil, nil, ErrFinalizeTimeout
}

// LoginWithCredentials performs the complete login flow in one call:
// credentials → Guard code → finalize.
//
// This is the recommended method for bot accounts with a device Steam Guard.
// The guardCode should be a fresh TOTP code generated from the account's shared secret.
//
// Example:
//
//	code, _ := auth.GenerateGuardCode(sharedSecret)
//	err := sess.LoginWithCredentials(ctx, "user", "pass", code)
func (s *SteamSession) LoginWithCredentials(ctx context.Context, accountName, password, guardCode string) error {
	err := s.WithCredentials(ctx, accountName, password)
	if err != nil {
		var guardErr *GuardRequiredError
		if !isGuardError(err, &guardErr) {
			return err
		}

		if guardErr.DeviceCode {
			if err := s.SubmitAuthCode(ctx, guardCode, GuardTypeDeviceCode); err != nil {
				return err
			}
		} else if guardErr.EmailCode {
			return fmt.Errorf("steamkit/auth: email Guard code required but device code provided: %w", ErrGuardRequired)
		} else {
			return fmt.Errorf("steamkit/auth: unsupported Guard confirmation type: %w", err)
		}
	}

	_, _, err = s.Finalize(ctx, 0)
	return err
}

// --- Token Management ---

// RefreshAccessToken requests a new access token from Steam using the refresh token.
//
// Use this when the current access token has expired. The refresh token has a much
// longer lifetime (~200 days) than access tokens (~24 hours).
//
// Returns ErrNoRefreshToken if no refresh token is available.
func (s *SteamSession) RefreshAccessToken(ctx context.Context) (*SteamToken, error) {
	if s.refreshToken == nil {
		return nil, ErrNoRefreshToken
	}

	resp, err := s.api.GenerateAccessTokenForApp(ctx, s.refreshToken.Raw, s.steamID)
	if err != nil {
		return nil, fmt.Errorf("steamkit/auth: failed to refresh access token: %w", err)
	}

	token, err := ParseToken(resp.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("steamkit/auth: failed to parse new access token: %w", err)
	}

	s.accessToken = token
	s.api.AccessToken = token.Raw

	// Check if Steam issued a new refresh token too
	if resp.RefreshToken != "" && resp.RefreshToken != s.refreshToken.Raw {
		newRefresh, err := ParseToken(resp.RefreshToken)
		if err == nil {
			s.refreshToken = newRefresh
		}
	}

	return token, nil
}

// --- Web Cookies ---

// Cookie represents a Steam web session cookie.
type Cookie struct {
	Domain string
	Name   string
	Value  string
}

// ObtainCookies generates web session cookies for all Steam domains.
//
// This method:
//  1. Generates a random sessionid if one doesn't exist
//  2. Uses the refresh token to finalize login via Steam's JWT flow
//  3. Sets sessionid, steamLoginSecure cookies on all Steam domains
//
// Returns the list of cookies set. These cookies are also stored in the
// session's internal HTTP client cookie jar, so subsequent requests via
// APIClient().HTTPClient() will include them automatically.
//
// Requires a valid refresh token (from Finalize or WithRefreshToken).
func (s *SteamSession) ObtainCookies(ctx context.Context) ([]Cookie, error) {
	if s.refreshToken == nil {
		return nil, ErrNoRefreshToken
	}

	// Ensure we have a valid access token
	if s.accessToken == nil || s.accessToken.Expired() {
		if _, err := s.RefreshAccessToken(ctx); err != nil {
			return nil, fmt.Errorf("steamkit/auth: failed to refresh access token for cookies: %w", err)
		}
	}

	// Generate session ID
	if s.sessionID == "" {
		s.sessionID = generateSessionID()
	}

	var cookies []Cookie

	// Set cookies on all Steam domains
	for _, domain := range steamapi.AllDomainURLs {
		// sessionid cookie
		s.api.SetCookie(domain, "sessionid", s.sessionID)
		cookies = append(cookies, Cookie{
			Domain: domain,
			Name:   "sessionid",
			Value:  s.sessionID,
		})

		// steamLoginSecure cookie (SteamID64||JWT)
		cookieValue := s.accessToken.CookieValue()
		s.api.SetCookie(domain, "steamLoginSecure", cookieValue)
		cookies = append(cookies, Cookie{
			Domain: domain,
			Name:   "steamLoginSecure",
			Value:  cookieValue,
		})
	}

	return cookies, nil
}

// CookiesValid checks whether the current web session cookies are still valid.
// Returns false if cookies haven't been obtained or the access token has expired.
func (s *SteamSession) CookiesValid() bool {
	if s == nil || s.accessToken == nil {
		return false
	}
	return !s.accessToken.Expired() && s.sessionID != ""
}

// --- Login Finalization via Steam Web (JWT/FinalizeLogin) ---

// FinalizeLoginViaWeb performs the full Steam web login finalization flow.
//
// This is the more robust login method that closely mirrors what aiosteampy does:
//  1. POST to login.steampowered.com/jwt/finalizelogin with the refresh token
//  2. Follow the transfer_info URLs to set cookies on all Steam domains
//  3. Extract and store the resulting auth cookies
//
// Use this instead of ObtainCookies when you need cookies that behave exactly
// like a real browser login (important for some community endpoints).
func (s *SteamSession) FinalizeLoginViaWeb(ctx context.Context) ([]Cookie, error) {
	if s.refreshToken == nil {
		return nil, ErrNoRefreshToken
	}

	// Generate session ID
	if s.sessionID == "" {
		s.sessionID = generateSessionID()
	}

	// Step 1: POST to jwt/finalizelogin
	data := url.Values{
		"nonce":     {s.refreshToken.Raw},
		"sessionid": {s.sessionID},
		"redir":     {steamapi.CommunityURL + "/login/home/?goto="},
	}

	headers := http.Header{}
	headers.Set("Accept", "application/json, text/plain, */*")
	headers.Set("Referer", steamapi.CommunityURL + "/")
	headers.Set("Origin", steamapi.CommunityURL)

	resp, err := s.api.PostForm(ctx, steamapi.LoginURL+"/jwt/finalizelogin", data, headers)
	if err != nil {
		return nil, fmt.Errorf("steamkit/auth: finalize login request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("steamkit/auth: failed to read finalize response: %w", err)
	}

	var loginResp struct {
		SteamID      string `json:"steamID"`
		Error        string `json:"error"`
		TransferInfo []struct {
			URL    string            `json:"url"`
			Params map[string]string `json:"params"`
		} `json:"transfer_info"`
	}

	if err := json.Unmarshal(body, &loginResp); err != nil {
		return nil, fmt.Errorf("steamkit/auth: failed to parse finalize response: %w", err)
	}

	if loginResp.Error != "" {
		return nil, fmt.Errorf("steamkit/auth: finalize login error: %s", loginResp.Error)
	}

	if len(loginResp.TransferInfo) == 0 || loginResp.SteamID == "" {
		return nil, fmt.Errorf("steamkit/auth: malformed finalize response (no transfer_info or steamID)")
	}

	// Step 2: Perform transfers to set cookies on all Steam domains
	for _, transfer := range loginResp.TransferInfo {
		transferData := url.Values{}
		for k, v := range transfer.Params {
			transferData.Set(k, v)
		}
		transferData.Set("steamID", loginResp.SteamID)

		transferResp, err := s.api.PostForm(ctx, transfer.URL, transferData, headers)
		if err != nil {
			// Non-fatal: some transfers may fail, but cookies may already be set
			continue
		}
		transferResp.Body.Close()
	}

	// Step 3: Set sessionid cookies on all domains and collect results
	var cookies []Cookie
	for _, domain := range steamapi.AllDomainURLs {
		s.api.SetCookie(domain, "sessionid", s.sessionID)
		cookies = append(cookies, Cookie{
			Domain: domain,
			Name:   "sessionid",
			Value:  s.sessionID,
		})

		// Find the steamLoginSecure cookie that the transfer set
		for _, c := range s.api.GetCookies(domain) {
			if c.Name == "steamLoginSecure" {
				cookies = append(cookies, Cookie{
					Domain: domain,
					Name:   c.Name,
					Value:  c.Value,
				})

				// Parse and store the access token from the community domain
				if strings.Contains(domain, "steamcommunity") {
					if token, err := ParseTokenFromCookie(c.Value); err == nil {
						s.accessToken = token
						s.api.AccessToken = token.Raw
					}
				}
			}
		}
	}

	return cookies, nil
}

// --- Serialization ---

// SerializedSession contains all data needed to restore a session.
type SerializedSession struct {
	Platform     string `json:"platform"`
	SteamID      string `json:"steam_id"`
	AccountName  string `json:"account_name"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	SessionID    string `json:"session_id,omitempty"`
}

// Serialize exports the session state to a JSON-safe struct.
// Use this to persist session data (e.g., to a file or database) and
// restore it later with DeserializeSession.
//
// Only tokens and identity are serialized — the HTTP client state
// (cookies, connections) is NOT included. Call ObtainCookies after
// deserialization to restore the cookie jar.
func (s *SteamSession) Serialize() *SerializedSession {
	result := &SerializedSession{
		Platform:    s.platform.String(),
		SteamID:     s.SteamID(),
		AccountName: s.accountName,
		SessionID:   s.sessionID,
	}
	if s.accessToken != nil {
		result.AccessToken = s.accessToken.Raw
	}
	if s.refreshToken != nil {
		result.RefreshToken = s.refreshToken.Raw
	}
	return result
}

// DeserializeSession restores a session from serialized data.
//
// This creates a new session with the saved tokens. The HTTP cookie jar
// will be empty — call ObtainCookies to restore web cookies.
//
// Example:
//
//	sess, err := auth.DeserializeSession(savedData)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	cookies, err := sess.ObtainCookies(ctx)
func DeserializeSession(data *SerializedSession) (*SteamSession, error) {
	var opts []SessionOption

	if data.RefreshToken != "" {
		opts = append(opts, WithRefreshToken(data.RefreshToken))
	}
	if data.AccessToken != "" {
		opts = append(opts, WithAccessToken(data.AccessToken))
	}

	sess, err := NewSession(opts...)
	if err != nil {
		return nil, err
	}

	sess.accountName = data.AccountName
	sess.sessionID = data.SessionID

	return sess, nil
}

// --- Helpers ---

// generateSessionID creates a random hex string matching Steam's sessionid format.
// Steam uses a 24-character hex string (12 random bytes).
func generateSessionID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// parseAllowedConfirmations converts the API response into a GuardRequiredError.
// Returns nil if no Guard confirmation is required (GuardTypeNone is allowed).
func parseAllowedConfirmations(confs []steamapi.AllowedConfirmation) *GuardRequiredError {
	if len(confs) == 0 {
		return nil
	}

	// Check if "None" is in the list — means no confirmation needed
	for _, conf := range confs {
		if conf.ConfirmationType == steamapi.GuardTypeNoneValue {
			return nil
		}
	}

	guardErr := &GuardRequiredError{}

	for _, conf := range confs {
		switch conf.ConfirmationType {
		case steamapi.GuardTypeDeviceCodeValue:
			guardErr.DeviceCode = true
			guardErr.AllowedTypes = append(guardErr.AllowedTypes, GuardTypeDeviceCode)
		case steamapi.GuardTypeEmailCodeValue:
			guardErr.EmailCode = true
			guardErr.AllowedTypes = append(guardErr.AllowedTypes, GuardTypeEmailCode)
		case steamapi.GuardTypeDeviceConfirmationValue:
			guardErr.DeviceConfirmation = true
			guardErr.AllowedTypes = append(guardErr.AllowedTypes, GuardTypeDeviceConfirmation)
		case steamapi.GuardTypeEmailConfirmationValue:
			guardErr.EmailConfirmation = true
			guardErr.AllowedTypes = append(guardErr.AllowedTypes, GuardTypeEmailConfirmation)
		}
	}

	return guardErr
}

// isGuardError is a helper to check if an error is a GuardRequiredError using errors.As.
func isGuardError(err error, target **GuardRequiredError) bool {
	if err == nil {
		return false
	}
	// Direct type assertion first (faster path)
	if ge, ok := err.(*GuardRequiredError); ok {
		*target = ge
		return true
	}
	return false
}
