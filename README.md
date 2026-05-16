# go-steamkit

A well-documented Go client for Steam Web API interactions, built for bots and automation.

`go-steamkit` provides authentication, trade management, and inventory fetching over HTTPS using Steam's public Web API — no Connection Manager (CM) TCP protocol required.

## Installation

```bash
go get github.com/muhd-k/go-steamkit
```

## Features

- **Authentication** — Full Steam Guard login flow with TOTP support, JWT token management, session persistence, and web cookie handling
- **Trade Management** — Fetch, send, accept, decline, and cancel trade offers; mobile confirmation support via `identity_secret`
- **Inventory Fetching** — Auto-paginated inventory retrieval with optional asset properties (stickers, wear, float, etc.)
- **Pure HTTP** — No Steam CM TCP connection; everything runs over HTTPS
- **Zero external dependencies** — Standard library only

## Quick Start

### Login

```go
import (
    "context"
    "log"

    "github.com/muhd-k/go-steamkit/auth"
)

func main() {
    ctx := context.Background()

    sess, err := auth.NewSession()
    if err != nil {
        log.Fatal(err)
    }

    // Generate TOTP code from your maFile's shared_secret
    code, err := auth.GenerateGuardCode(sharedSecret)
    if err != nil {
        log.Fatal(err)
    }

    err = sess.LoginWithCredentials(ctx, username, password, code)
    if err != nil {
        log.Fatal(err)
    }

    // Obtain web cookies for community endpoints
    _, err = sess.ObtainCookies(ctx)
    if err != nil {
        log.Fatal(err)
    }
}
```

### Trade Offers

```go
import "github.com/muhd-k/go-steamkit/trade"

// API key is optional — if empty, the session's access token is used
client, err := trade.NewClient(sess, "")
if err != nil {
    log.Fatal(err)
}

offers, err := client.GetActiveOffers(ctx, true)
if err != nil {
    log.Fatal(err)
}
```

### Inventory

```go
import "github.com/muhd-k/go-steamkit/inventory"

client, err := inventory.NewClient(sess)
if err != nil {
    log.Fatal(err)
}

items, err := client.GetAllMyInventory(ctx, 730, 2, &inventory.GetInventoryOptions{
    IncludeProperties: true, // fetch stickers, wear, float, etc.
})
if err != nil {
    log.Fatal(err)
}
```

### Session Persistence

```go
// Save session
serialized := sess.Serialize()

// Restore later
sess, err := auth.DeserializeSession(serialized)
```

## Package Overview

| Package | Purpose |
|---------|---------|
| `auth` | Authentication, Steam Guard TOTP, JWT tokens, session serialization |
| `trade` | Trade offer management and mobile confirmations |
| `inventory` | Inventory fetching with pagination and asset properties |

## Running Tests

Unit tests (no external dependencies):

```bash
go test ./...
```

Integration tests (requires Steam credentials):

```bash
export STEAM_USERNAME=your_username
export STEAM_PASSWORD=your_password
export STEAM_SHARED_SECRET=your_shared_secret

go test ./auth -tags integration -v -run TestLiveLogin
go test ./trade -tags integration -v -run TestLiveTrade
go test ./inventory -tags integration -v -run TestLiveInventory
```

## Documentation

For a detailed breakdown of the internals and architecture, see the [deepwiki documentation](https://deepwiki.com/muhd-k/go-steamkit).

## References

This library draws conceptual inspiration from:

- **[aiosteampy](https://github.com/somespecialone/aiosteampy)** — Python async Steam client (session lifecycle, documentation patterns)
- **[go-steam](https://github.com/kolosok86/go-steam)** — Go Steam client by kolosok86 (TOTP algorithm, protobuf references)

## License

MIT
