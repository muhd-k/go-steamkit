// Package auth provides Steam authentication, session management, and Steam Guard TOTP.
//
// This package handles the full login lifecycle using Steam's Web API (IAuthenticationService),
// without requiring the Steam CM TCP protocol. It supports:
//
//   - Credential-based login with Steam Guard (device/email code) confirmation
//   - JWT token management (access + refresh tokens)
//   - Session cookie management for steamcommunity.com and store.steampowered.com
//   - Session serialization/deserialization for persistence across restarts
//   - Steam Guard TOTP code generation from shared secrets
//
// # Login Flow
//
// The login process follows Steam's IAuthenticationService flow:
//
//  1. Fetch RSA public key for the account
//  2. Encrypt password with RSA
//  3. Begin auth session via credentials
//  4. Submit Steam Guard code (if required)
//  5. Poll for session status until tokens are issued
//  6. Obtain web cookies using the refresh token
//
// # Usage Patterns
//
// Full login with device Steam Guard code:
//
//	sess, _ := auth.NewSession()
//	code, _ := auth.GenerateGuardCode(sharedSecret)
//	err := sess.LoginWithCredentials(ctx, "user", "pass", code)
//	cookies, err := sess.ObtainCookies(ctx)
//
// Step-by-step login (useful when Guard type is unknown):
//
//	sess, _ := auth.NewSession()
//	err := sess.WithCredentials(ctx, "user", "pass")
//	if errors.Is(err, auth.ErrGuardRequired) {
//	    var guardErr *auth.GuardRequiredError
//	    errors.As(err, &guardErr)
//	    if guardErr.DeviceCode {
//	        code, _ := auth.GenerateGuardCode(sharedSecret)
//	        sess.SubmitAuthCode(ctx, code, auth.GuardTypeDevice)
//	    }
//	}
//	tokens, err := sess.Finalize(ctx, 0)
//	cookies, err := sess.ObtainCookies(ctx)
//
// Restore session from saved tokens:
//
//	sess, err := auth.NewSession(auth.WithRefreshToken(savedRefreshToken))
//	cookies, err := sess.ObtainCookies(ctx)
package auth
