//go:build integration

package auth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// Run with: go test ./auth -tags integration -v -run TestLiveLogin
//
// Required environment variables:
//   STEAM_USERNAME, STEAM_PASSWORD, STEAM_SHARED_SECRET

func TestLiveLogin(t *testing.T) {
	username := os.Getenv("STEAM_USERNAME")
	password := os.Getenv("STEAM_PASSWORD")
	sharedSecret := os.Getenv("STEAM_SHARED_SECRET")

	if username == "" || password == "" || sharedSecret == "" {
		t.Skip("STEAM_USERNAME, STEAM_PASSWORD, STEAM_SHARED_SECRET must be set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Step 1: Create session
	sess, err := NewSession()
	if err != nil {
		t.Fatalf("NewSession() error: %v", err)
	}
	t.Log("✓ Session created")

	// Step 2: Generate TOTP code
	code, err := GenerateGuardCode(sharedSecret)
	if err != nil {
		t.Fatalf("GenerateGuardCode() error: %v", err)
	}
	t.Logf("✓ Generated Guard code: %s", code)

	// Step 3: Login with credentials
	err = sess.LoginWithCredentials(ctx, username, password, code)
	if err != nil {
		t.Fatalf("LoginWithCredentials() error: %v", err)
	}
	t.Logf("✓ Login successful! SteamID: %s, Account: %s", sess.SteamID(), sess.AccountName())

	// Step 4: Verify tokens
	if sess.AccessToken() == nil {
		t.Fatal("Access token is nil after login")
	}
	if sess.RefreshToken() == nil {
		t.Fatal("Refresh token is nil after login")
	}
	t.Logf("✓ Access token: %s", sess.AccessToken())
	t.Logf("✓ Refresh token: %s", sess.RefreshToken())

	// Step 5: Obtain cookies
	cookies, err := sess.ObtainCookies(ctx)
	if err != nil {
		t.Fatalf("ObtainCookies() error: %v", err)
	}
	t.Logf("✓ Obtained %d cookies", len(cookies))
	for _, c := range cookies {
		t.Logf("  Cookie: %s @ %s = %s...", c.Name, c.Domain, truncate(c.Value, 40))
	}

	// Step 6: Verify cookies work — fetch profile page
	profileURL := fmt.Sprintf("https://steamcommunity.com/profiles/%s", sess.SteamID())
	httpClient := sess.APIClient().HTTPClient()
	req, _ := http.NewRequestWithContext(ctx, "GET", profileURL, nil)
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("Profile request failed: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	t.Logf("✓ Profile page status: %d, body length: %d", resp.StatusCode, len(body))

	if resp.StatusCode != 200 {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}

	// Check that we got an authenticated page (should contain the account name or profile data)
	if strings.Contains(bodyStr, "Sign In") && !strings.Contains(bodyStr, username) {
		t.Log("⚠ Page appears to be unauthenticated (contains 'Sign In')")
	} else {
		t.Log("✓ Profile page appears authenticated")
	}

	// Step 7: Test serialization round-trip
	serialized := sess.Serialize()
	t.Logf("✓ Serialized session: SteamID=%s, Platform=%s", serialized.SteamID, serialized.Platform)

	restored, err := DeserializeSession(serialized)
	if err != nil {
		t.Fatalf("DeserializeSession() error: %v", err)
	}
	if restored.SteamID() != sess.SteamID() {
		t.Errorf("Restored SteamID mismatch: %s vs %s", restored.SteamID(), sess.SteamID())
	}
	t.Log("✓ Serialization round-trip OK")

	// Step 8: Test token refresh
	newToken, err := restored.RefreshAccessToken(ctx)
	if err != nil {
		t.Fatalf("RefreshAccessToken() error: %v", err)
	}
	t.Logf("✓ Refreshed access token: %s", newToken)

	t.Log("\n=== ALL INTEGRATION TESTS PASSED ===")
}

// TestLiveLoginStepByStep tests the granular step-by-step login flow
func TestLiveLoginStepByStep(t *testing.T) {
	username := os.Getenv("STEAM_USERNAME")
	password := os.Getenv("STEAM_PASSWORD")
	sharedSecret := os.Getenv("STEAM_SHARED_SECRET")

	if username == "" || password == "" || sharedSecret == "" {
		t.Skip("STEAM_USERNAME, STEAM_PASSWORD, STEAM_SHARED_SECRET must be set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sess, err := NewSession()
	if err != nil {
		t.Fatalf("NewSession() error: %v", err)
	}

	// Step 1: WithCredentials should return GuardRequiredError
	err = sess.WithCredentials(ctx, username, password)
	if err == nil {
		t.Log("✓ WithCredentials succeeded without Guard (unusual but valid)")
	} else {
		var guardErr *GuardRequiredError
		if !isGuardError(err, &guardErr) {
			t.Fatalf("WithCredentials() unexpected error: %v", err)
		}
		t.Logf("✓ Guard required: DeviceCode=%v, EmailCode=%v", guardErr.DeviceCode, guardErr.EmailCode)

		if !guardErr.DeviceCode {
			t.Fatal("Expected DeviceCode guard type")
		}

		// Step 2: Submit device code
		code, _ := GenerateGuardCode(sharedSecret)
		t.Logf("  Submitting code: %s", code)
		err = sess.SubmitAuthCode(ctx, code, GuardTypeDeviceCode)
		if err != nil {
			t.Fatalf("SubmitAuthCode() error: %v", err)
		}
		t.Log("✓ Guard code accepted")
	}

	// Step 3: Finalize
	accessToken, refreshToken, err := sess.Finalize(ctx, 0)
	if err != nil {
		t.Fatalf("Finalize() error: %v", err)
	}
	t.Logf("✓ Finalized: access=%s, refresh=%s", accessToken, refreshToken)

	// Step 4: ObtainCookies
	cookies, err := sess.ObtainCookies(ctx)
	if err != nil {
		t.Fatalf("ObtainCookies() error: %v", err)
	}
	t.Logf("✓ Got %d cookies", len(cookies))

	t.Log("\n=== STEP-BY-STEP TEST PASSED ===")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
