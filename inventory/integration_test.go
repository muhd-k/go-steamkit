//go:build integration

package inventory

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/skinflippa/go-steamkit/auth"
)

// Run with: go test ./inventory -tags integration -v -run TestLiveInventory
//
// Required environment variables:
//   STEAM_USERNAME, STEAM_PASSWORD, STEAM_SHARED_SECRET

func TestLiveInventory(t *testing.T) {
	username := os.Getenv("STEAM_USERNAME")
	password := os.Getenv("STEAM_PASSWORD")
	sharedSecret := os.Getenv("STEAM_SHARED_SECRET")

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

	// Step 3: Create inventory client
	client, err := NewClient(sess)
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}

	// Step 4: Fetch inventory without properties
	t.Log("Fetching inventory without properties...")
	items, err := client.GetAllMyInventory(ctx, 730, 2, nil)
	if err != nil {
		t.Fatalf("GetAllMyInventory() error: %v", err)
	}
	t.Logf("Fetched %d items", len(items))

	if len(items) > 0 {
		item := items[0]
		t.Logf("First item: AssetID=%d, ClassID=%d, InstanceID=%d", item.AssetID, item.ClassID, item.InstanceID)
		if item.Description != nil {
			t.Logf("  Description: Name=%s, MarketHashName=%s", item.Description.Name, item.Description.MarketHashName)
			t.Logf("  Tradable=%v, Marketable=%v, Commodity=%v", item.Description.Tradable, item.Description.Marketable, item.Description.Commodity)
		} else {
			t.Log("  No description linked")
		}
		if len(item.Properties) > 0 {
			t.Errorf("Expected no properties without IncludeProperties, got %d", len(item.Properties))
		}
		if len(item.Accessories) > 0 {
			t.Errorf("Expected no accessories without IncludeProperties, got %d", len(item.Accessories))
		}
	}

	// Step 5: Fetch inventory with properties
	t.Log("Fetching inventory with properties...")
	itemsWithProps, err := client.GetAllMyInventory(ctx, 730, 2, &GetInventoryOptions{IncludeProperties: true})
	if err != nil {
		t.Fatalf("GetAllMyInventory(with properties) error: %v", err)
	}
	t.Logf("Fetched %d items with properties", len(itemsWithProps))

	if len(itemsWithProps) > 0 {
		item := itemsWithProps[0]
		t.Logf("First item: AssetID=%d, Properties=%d, Accessories=%d", item.AssetID, len(item.Properties), len(item.Accessories))
		for i, p := range item.Properties {
			t.Logf("  Property %d: id=%d, value=%s", i, p.PropertyID, p.Value)
		}
		for i, a := range item.Accessories {
			t.Logf("  Accessory %d: classid=%d, parent_props=%d, standalone_props=%d",
				i, a.ClassID, len(a.ParentRelationshipProperties), len(a.StandaloneProperties))
		}
	}

	t.Log("\n=== INVENTORY INTEGRATION TEST PASSED ===")
}
