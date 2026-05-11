// Package steamkit provides a well-documented Go client for Steam Web API interactions.
//
// steamkit is designed for bot and automation use-cases — trading, inventory management,
// and authentication — without requiring the Steam CM (Connection Manager) TCP protocol.
// All communication happens over HTTPS using Steam's public Web API.
//
// # Architecture
//
// The package is organized into focused sub-packages:
//
//   - [github.com/skinflippa/go-steamkit/auth] — Authentication, session management, and Steam Guard TOTP
//   - github.com/skinflippa/go-steamkit/trade (Phase 2) — Trade offer management
//   - github.com/skinflippa/go-steamkit/inventory (Phase 3) — Inventory fetching
//
// # Quick Start
//
// Login with credentials and a Steam Guard device code:
//
//	session, err := auth.NewSession()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Generate TOTP code from your maFile's shared_secret
//	code, err := auth.GenerateGuardCode(sharedSecret)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	err = session.LoginWithCredentials(ctx, "username", "password", code)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Session is now authenticated — obtain web cookies for further API calls
//	cookies, err := session.ObtainCookies(ctx)
//
// # References
//
// This library draws conceptual inspiration from:
//   - [aiosteampy] — Python async Steam client (session lifecycle, documentation patterns)
//   - [go-steam] — Go Steam client by kolosok86 (TOTP algorithm, protobuf references)
//
// [aiosteampy]: https://github.com/somespecialone/aiosteampy
// [go-steam]: https://github.com/kolosok86/go-steam
package steamkit
