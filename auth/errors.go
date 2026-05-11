package auth

import (
	"errors"
	"fmt"
)

// Sentinel errors for the auth package.
// Use errors.Is to check for these in your error handling.
var (
	// ErrBadCredentials indicates that the username or password was rejected by Steam.
	ErrBadCredentials = errors.New("steamkit/auth: invalid username or password")

	// ErrTooManyAttempts indicates that Steam has rate-limited login attempts.
	// Wait before retrying — typically a few minutes, sometimes longer.
	ErrTooManyAttempts = errors.New("steamkit/auth: rate limited, too many login attempts")

	// ErrAuthCodeExpired indicates that the submitted Steam Guard code has expired.
	// Generate a fresh TOTP code and retry.
	ErrAuthCodeExpired = errors.New("steamkit/auth: Steam Guard code has expired")

	// ErrAuthCodeInvalid indicates that the submitted Steam Guard code was incorrect.
	ErrAuthCodeInvalid = errors.New("steamkit/auth: Steam Guard code is invalid")

	// ErrGuardRequired is a sentinel for GuardRequiredError.
	// Use errors.As to extract the full GuardRequiredError with confirmation details.
	ErrGuardRequired = errors.New("steamkit/auth: Steam Guard confirmation required")

	// ErrSessionNotReady indicates the session has not completed the login flow.
	// Call WithCredentials or LoginWithCredentials before attempting operations
	// that require an authenticated session.
	ErrSessionNotReady = errors.New("steamkit/auth: session not initialized")

	// ErrNoRefreshToken indicates a refresh token is required but not available.
	ErrNoRefreshToken = errors.New("steamkit/auth: refresh token not available")

	// ErrTokenExpired indicates a token has expired and cannot be used.
	ErrTokenExpired = errors.New("steamkit/auth: token has expired")

	// ErrFinalizeTimeout indicates that polling for auth session status timed out.
	ErrFinalizeTimeout = errors.New("steamkit/auth: timeout waiting for auth session finalization")
)

// GuardType identifies the type of Steam Guard confirmation required.
type GuardType int

const (
	// GuardTypeNone indicates no Guard confirmation is needed.
	GuardTypeNone GuardType = iota

	// GuardTypeDeviceCode requires a TOTP code from the Steam Mobile App authenticator.
	// Generate this using [GenerateGuardCode] or [Signer.Code].
	GuardTypeDeviceCode

	// GuardTypeEmailCode requires a code sent to the account's email address.
	GuardTypeEmailCode

	// GuardTypeDeviceConfirmation requires tapping "Approve" in the Steam Mobile App.
	GuardTypeDeviceConfirmation

	// GuardTypeEmailConfirmation requires clicking a link in a confirmation email.
	GuardTypeEmailConfirmation
)

// String returns a human-readable name for the guard type.
func (g GuardType) String() string {
	switch g {
	case GuardTypeNone:
		return "None"
	case GuardTypeDeviceCode:
		return "DeviceCode"
	case GuardTypeEmailCode:
		return "EmailCode"
	case GuardTypeDeviceConfirmation:
		return "DeviceConfirmation"
	case GuardTypeEmailConfirmation:
		return "EmailConfirmation"
	default:
		return fmt.Sprintf("Unknown(%d)", int(g))
	}
}

// GuardRequiredError is returned when Steam requires additional confirmation
// to complete the login. Inspect the AllowedTypes field to determine what
// confirmation methods are available.
//
// Example:
//
//	err := sess.WithCredentials(ctx, user, pass)
//	var guardErr *auth.GuardRequiredError
//	if errors.As(err, &guardErr) {
//	    if guardErr.DeviceCode {
//	        // Submit a TOTP device code
//	    }
//	}
type GuardRequiredError struct {
	// AllowedTypes lists all Guard confirmation types accepted by Steam for this login.
	AllowedTypes []GuardType

	// DeviceCode is true if Steam accepts a TOTP device code (most common for bot accounts).
	DeviceCode bool

	// EmailCode is true if Steam accepts a code sent via email.
	EmailCode bool

	// DeviceConfirmation is true if Steam accepts a tap-to-approve from the mobile app.
	DeviceConfirmation bool

	// EmailConfirmation is true if Steam accepts an email confirmation link.
	EmailConfirmation bool
}

func (e *GuardRequiredError) Error() string {
	return ErrGuardRequired.Error()
}

// Unwrap allows errors.Is(err, ErrGuardRequired) to work.
func (e *GuardRequiredError) Unwrap() error {
	return ErrGuardRequired
}

// EResultError represents a Steam Web API error with a numeric result code.
// Steam returns these in the X-eresult response header.
type EResultError struct {
	// Result is the numeric Steam EResult code.
	Result int

	// Message is the optional error message from the X-error_message header.
	Message string
}

func (e *EResultError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("steamkit/auth: Steam API error %d: %s", e.Result, e.Message)
	}
	return fmt.Sprintf("steamkit/auth: Steam API error %d", e.Result)
}

// Common EResult codes used in the auth flow.
// See https://steamerrors.com for a complete reference.
const (
	EResultOK                          = 1
	EResultFail                        = 2
	EResultInvalidPassword             = 5
	EResultAccessDenied                = 15
	EResultExpired                     = 27
	EResultDuplicateRequest            = 29
	EResultInvalidLoginAuthCode        = 65
	EResultExpiredLoginAuthCode        = 71
	EResultRateLimitExceeded           = 84
	EResultAccountLogonDeniedNeed2FA   = 85
	EResultTwoFactorCodeMismatch      = 88
)
