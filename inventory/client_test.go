package inventory

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/skinflippa/go-steamkit/auth"
)

const testRefreshJWT = "eyJ0eXAiOiAiSldUIiwgImFsZyI6ICJFZERTQSJ9.eyJpc3MiOiAic3RlYW0iLCAic3ViIjogIjc2NTYxMTk4MDEyMzQ1Njc4IiwgImF1ZCI6IFsiZGVyaXZlIl0sICJleHAiOiAyMDAwMDAwMDAwLCAibmJmIjogMTcwMDAwMDAwMCwgImlhdCI6IDE3MDAwMDAwMDAsICJqdGkiOiAicmVmcmVzaDEyMyIsICJwZXIiOiAwfQ.ZmFrZXNpZzEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2"

func TestGetAllInventory_Basic(t *testing.T) {
	body := `{
		"success": true,
		"total_inventory_count": 2,
		"assets": [
			{"appid":"730","contextid":"2","assetid":"1001","classid":"101","instanceid":"1","amount":"1"},
			{"appid":"730","contextid":"2","assetid":"1002","classid":"102","instanceid":"1","amount":"1"}
		],
		"descriptions": [
			{"appid":"730","classid":"101","instanceid":"1","name":"AK-47","market_name":"AK-47 | Redline","market_hash_name":"AK-47 | Redline (Field-Tested)","icon_url":"icon1","tradable":1,"marketable":1,"commodity":0},
			{"appid":"730","classid":"102","instanceid":"1","name":"M4A4","market_name":"M4A4 | Howl","market_hash_name":"M4A4 | Howl (Factory New)","icon_url":"icon2","tradable":1,"marketable":1,"commodity":0}
		],
		"last_assetid": "",
		"more_items": 0
	}`

	client := newTestClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if !strings.Contains(r.URL.Path, "/inventory/") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		return jsonResponse(body), nil
	}))

	items, err := client.GetAllInventory(context.Background(), 76561198012345678, 730, 2, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	if items[0].AssetID != 1001 {
		t.Errorf("expected assetid 1001, got %d", items[0].AssetID)
	}
	if items[0].Description == nil {
		t.Fatal("expected description for first item")
	}
	if items[0].Description.Name != "AK-47" {
		t.Errorf("expected name AK-47, got %s", items[0].Description.Name)
	}
	if items[0].Description.MarketHashName != "AK-47 | Redline (Field-Tested)" {
		t.Errorf("unexpected market hash name: %s", items[0].Description.MarketHashName)
	}
	if !items[0].Description.Tradable {
		t.Error("expected first item to be tradable")
	}

	if items[1].Description == nil {
		t.Fatal("expected description for second item")
	}
	if items[1].Description.Name != "M4A4" {
		t.Errorf("expected name M4A4, got %s", items[1].Description.Name)
	}
}

func TestGetAllInventory_Pagination(t *testing.T) {
	page1 := `{
		"success": true,
		"total_inventory_count": 3,
		"assets": [
			{"appid":"730","contextid":"2","assetid":"1001","classid":"101","instanceid":"1","amount":"1"}
		],
		"descriptions": [
			{"appid":"730","classid":"101","instanceid":"1","name":"Item1","icon_url":"icon1","tradable":1,"marketable":1,"commodity":0}
		],
		"last_assetid": "1002",
		"more_items": 1
	}`
	page2 := `{
		"success": true,
		"total_inventory_count": 3,
		"assets": [
			{"appid":"730","contextid":"2","assetid":"1002","classid":"102","instanceid":"1","amount":"1"},
			{"appid":"730","contextid":"2","assetid":"1003","classid":"103","instanceid":"1","amount":"1"}
		],
		"descriptions": [
			{"appid":"730","classid":"102","instanceid":"1","name":"Item2","icon_url":"icon2","tradable":1,"marketable":1,"commodity":0},
			{"appid":"730","classid":"103","instanceid":"1","name":"Item3","icon_url":"icon3","tradable":1,"marketable":1,"commodity":0}
		],
		"last_assetid": "",
		"more_items": 0
	}`

	callCount := 0
	client := newTestClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		callCount++
		startAssetID := r.URL.Query().Get("start_assetid")
		if startAssetID == "" {
			return jsonResponse(page1), nil
		}
		if startAssetID == "1002" {
			return jsonResponse(page2), nil
		}
		t.Fatalf("unexpected start_assetid: %s", startAssetID)
		return nil, nil
	}))

	items, err := client.GetAllInventory(context.Background(), 76561198012345678, 730, 2, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[0].AssetID != 1001 || items[1].AssetID != 1002 || items[2].AssetID != 1003 {
		t.Errorf("unexpected asset IDs: %v", []uint64{items[0].AssetID, items[1].AssetID, items[2].AssetID})
	}
	if callCount != 2 {
		t.Errorf("expected 2 requests, got %d", callCount)
	}
}

func TestGetAllInventory_Empty(t *testing.T) {
	body := `{"success": true, "total_inventory_count": 0, "assets": [], "descriptions": [], "last_assetid": "", "more_items": 0}`

	client := newTestClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(body), nil
	}))

	items, err := client.GetAllInventory(context.Background(), 76561198012345678, 730, 2, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestGetAllInventory_Private(t *testing.T) {
	client := newTestClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 403, Body: io.NopCloser(strings.NewReader("Forbidden"))}, nil
	}))

	_, err := client.GetAllInventory(context.Background(), 76561198012345678, 730, 2, nil)
	if err != ErrInventoryPrivate {
		t.Fatalf("expected ErrInventoryPrivate, got %v", err)
	}
}

func TestGetAllInventory_IncludeProperties(t *testing.T) {
	body := `{
		"success": true,
		"total_inventory_count": 1,
		"assets": [
			{"appid":"730","contextid":"2","assetid":"1001","classid":"101","instanceid":"1","amount":"1"}
		],
		"descriptions": [
			{"appid":"730","classid":"101","instanceid":"1","name":"AK-47","icon_url":"icon1","tradable":1,"marketable":1,"commodity":0}
		],
		"asset_properties": [
			{
				"assetid": "1001",
				"asset_properties": [
					{"propertyid": 1, "value": "0.123456789"}
				],
				"asset_accessories": [
					{
						"classid": 201,
						"parent_relationship_properties": [
							{"propertyid": 10, "value": "sticker1"}
						],
						"standalone_properties": [
							{"propertyid": 11, "value": "sticker2"}
						]
					}
				]
			}
		],
		"last_assetid": "",
		"more_items": 0
	}`

	var rawPropParam string
	client := newTestClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		rawPropParam = r.URL.Query().Get("raw_asset_properties")
		return jsonResponse(body), nil
	}))

	items, err := client.GetAllInventory(context.Background(), 76561198012345678, 730, 2, &GetInventoryOptions{IncludeProperties: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rawPropParam != "1" {
		t.Errorf("expected raw_asset_properties=1, got %q", rawPropParam)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if len(items[0].Properties) != 1 {
		t.Fatalf("expected 1 property, got %d", len(items[0].Properties))
	}
	if items[0].Properties[0].Value != "0.123456789" {
		t.Errorf("unexpected property value: %s", items[0].Properties[0].Value)
	}
	if len(items[0].Accessories) != 1 {
		t.Fatalf("expected 1 accessory, got %d", len(items[0].Accessories))
	}
	if items[0].Accessories[0].ClassID != 201 {
		t.Errorf("unexpected accessory classid: %d", items[0].Accessories[0].ClassID)
	}
	if len(items[0].Accessories[0].ParentRelationshipProperties) != 1 {
		t.Fatalf("expected 1 parent property, got %d", len(items[0].Accessories[0].ParentRelationshipProperties))
	}
	if len(items[0].Accessories[0].StandaloneProperties) != 1 {
		t.Fatalf("expected 1 standalone property, got %d", len(items[0].Accessories[0].StandaloneProperties))
	}
}

func TestGetAllMyInventory_MissingSteamID(t *testing.T) {
	sess, err := auth.DeserializeSession(&auth.SerializedSession{
		RefreshToken: testRefreshJWT,
		SessionID:    "session-123",
	})
	if err != nil {
		t.Fatalf("DeserializeSession() error: %v", err)
	}
	client, err := NewClient(sess)
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}

	_, err = client.GetAllMyInventory(context.Background(), 730, 2, nil)
	if err == nil {
		t.Fatal("expected error for missing steamID")
	}
}

func TestGetAllInventory_NoCookies(t *testing.T) {
	sess, err := auth.DeserializeSession(&auth.SerializedSession{
		RefreshToken: testRefreshJWT,
	})
	if err != nil {
		t.Fatalf("DeserializeSession() error: %v", err)
	}
	client, err := NewClient(sess)
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}

	_, err = client.GetAllInventory(context.Background(), 76561198012345678, 730, 2, nil)
	if err != ErrSessionCookiesRequired {
		t.Fatalf("expected ErrSessionCookiesRequired, got %v", err)
	}
}

func TestNewClient_NilSession(t *testing.T) {
	_, err := NewClient(nil)
	if err != ErrSessionRequired {
		t.Fatalf("expected ErrSessionRequired, got %v", err)
	}
}

func TestGetAllInventory_SteamError(t *testing.T) {
	body := `{"success": false, "error": "Internal Server Error"}`

	client := newTestClient(t, roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(body), nil
	}))

	_, err := client.GetAllInventory(context.Background(), 76561198012345678, 730, 2, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	se, ok := err.(*SteamError)
	if !ok {
		t.Fatalf("expected *SteamError, got %T", err)
	}
	if se.Message != "inventory error: Internal Server Error" {
		t.Errorf("unexpected error message: %s", se.Message)
	}
}

func newTestClient(t *testing.T, rt http.RoundTripper) *Client {
	t.Helper()

	sess, err := auth.DeserializeSession(&auth.SerializedSession{
		RefreshToken: testRefreshJWT,
		SessionID:    "session-123",
	})
	if err != nil {
		t.Fatalf("DeserializeSession() error: %v", err)
	}
	client, err := NewClient(sess)
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}
	client.http = &http.Client{Transport: rt}
	return client
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
