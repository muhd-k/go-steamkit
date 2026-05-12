//go:build integration

package trade

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/skinflippa/go-steamkit/auth"
)

// Run with: go test ./trade -tags integration -v -run TestLiveTrade
//
// Required environment variables:
//   STEAM_USERNAME, STEAM_PASSWORD, STEAM_SHARED_SECRET
//
// Optional:
//   STEAM_API_KEY — if provided, trade client will use it instead of access_token
//   STEAM_PARTNER_ID — a partner SteamID64 to test escrow duration (safe & read-only)

func TestLiveTrade(t *testing.T) {
	username := os.Getenv("STEAM_USERNAME")
	password := os.Getenv("STEAM_PASSWORD")
	sharedSecret := os.Getenv("STEAM_SHARED_SECRET")
	apiKey := os.Getenv("STEAM_API_KEY")

	if username == "" || password == "" || sharedSecret == "" {
		t.Skip("STEAM_USERNAME, STEAM_PASSWORD, STEAM_SHARED_SECRET must be set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Step 1: Login
	sess, err := auth.NewSession()
	if err != nil {
		t.Fatalf("NewSession() error: %v", err)
	}

	code, err := auth.GenerateGuardCode(sharedSecret)
	if err != nil {
		t.Fatalf("GenerateGuardCode() error: %v", err)
	}
	t.Logf("Generated Guard code: %s", code)

	err = sess.LoginWithCredentials(ctx, username, password, code)
	if err != nil {
		t.Fatalf("LoginWithCredentials() error: %v", err)
	}
	t.Logf("Login successful! SteamID: %s", sess.SteamID())

	// Step 2: Obtain cookies
	_, err = sess.ObtainCookies(ctx)
	if err != nil {
		t.Fatalf("ObtainCookies() error: %v", err)
	}
	t.Log("Cookies obtained")

	// Step 3: Create trade client (apiKey is optional)
	client, err := NewClient(sess, apiKey)
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}
	if apiKey != "" {
		t.Log("Trade client created with API key")
	} else {
		t.Log("Trade client created with access token")
	}

	// Step 4: Fetch active offers (read-only)
	t.Log("Fetching active trade offers...")
	offers, err := client.GetActiveOffers(ctx, true)
	if err != nil {
		t.Fatalf("GetActiveOffers() error: %v", err)
	}
	t.Logf("Active offers: %d sent, %d received", len(offers.Sent), len(offers.Received))

	if len(offers.Sent) > 0 {
		o := offers.Sent[0]
		t.Logf("First sent offer: ID=%d, State=%d, Partner=%d, ItemsToGive=%d, ItemsToReceive=%d",
			o.ID, o.State, o.OtherSteamID, len(o.ItemsToGive), len(o.ItemsToReceive))
	}
	if len(offers.Received) > 0 {
		o := offers.Received[0]
		t.Logf("First received offer: ID=%d, State=%d, Partner=%d, ItemsToGive=%d, ItemsToReceive=%d",
			o.ID, o.State, o.OtherSteamID, len(o.ItemsToGive), len(o.ItemsToReceive))
	}

	// Step 5: Escrow check with partner (read-only, optional)
	partnerIDStr := os.Getenv("STEAM_PARTNER_ID")
	if partnerIDStr != "" {
		var partnerID uint64
		_, err := fmt.Sscanf(partnerIDStr, "%d", &partnerID)
		if err != nil {
			t.Logf("Skipping escrow test: invalid STEAM_PARTNER_ID: %v", err)
		} else {
			t.Logf("Checking escrow duration with partner %d...", partnerID)
			duration, err := client.GetPartnerEscrowDuration(ctx, partnerID, "")
			if err != nil {
				t.Logf("GetPartnerEscrowDuration() error (partner may have escrow disabled): %v", err)
			} else {
				t.Logf("Escrow duration — Mine: %d days, Theirs: %d days", duration.DaysMyEscrow, duration.DaysTheirEscrow)
			}
		}
	}

	t.Log("\n=== TRADE INTEGRATION TEST PASSED ===")
}
