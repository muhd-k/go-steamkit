package trade

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/skinflippa/go-steamkit/auth"
)

const testRefreshJWT = "eyJ0eXAiOiAiSldUIiwgImFsZyI6ICJFZERTQSJ9.eyJpc3MiOiAic3RlYW0iLCAic3ViIjogIjc2NTYxMTk4MDEyMzQ1Njc4IiwgImF1ZCI6IFsiZGVyaXZlIl0sICJleHAiOiAyMDAwMDAwMDAwLCAibmJmIjogMTcwMDAwMDAwMCwgImlhdCI6IDE3MDAwMDAwMDAsICJqdGkiOiAicmVmcmVzaDEyMyIsICJwZXIiOiAwfQ.ZmFrZXNpZzEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2"

func TestGetOffer(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/IEconService/GetTradeOffer/v1" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("key"); got != "api-key" {
			t.Fatalf("key = %q", got)
		}
		if got := r.URL.Query().Get("tradeofferid"); got != "42" {
			t.Fatalf("tradeofferid = %q", got)
		}
		if got := r.URL.Query().Get("get_descriptions"); got != "1" {
			t.Fatalf("get_descriptions = %q", got)
		}

		return jsonResponse(`{"response":{"offer":{"tradeofferid":"42","accountid_other":123,"trade_offer_state":2},"descriptions":[{"appid":730,"classid":"1","instanceid":"2","name":"AK"}]}}`), nil
	})}

	client := &Client{
		apiKey:        "api-key",
		http:          httpClient,
		webAPIBaseURL: "https://api.test",
	}

	result, err := client.GetOffer(context.Background(), 42, true)
	if err != nil {
		t.Fatalf("GetOffer() error: %v", err)
	}
	if result.Offer.ID != 42 {
		t.Errorf("offer id = %d", result.Offer.ID)
	}
	if result.Offer.OtherSteamID != steamID64IndividualBase+123 {
		t.Errorf("other steam id = %d", result.Offer.OtherSteamID)
	}
	if len(result.Descriptions) != 1 || result.Descriptions[0].Name != "AK" {
		t.Fatalf("descriptions = %#v", result.Descriptions)
	}
}

func TestGetOfferWithAccessToken(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.Query().Get("access_token"); got == "" {
			t.Fatalf("expected access_token param, got none")
		}
		if got := r.URL.Query().Get("key"); got != "" {
			t.Fatalf("expected no key param, got %q", got)
		}
		return jsonResponse(`{"response":{"offer":{"tradeofferid":"42","accountid_other":123,"trade_offer_state":2}}}`), nil
	})}

	// Use an access token (web audience) so authParams falls back to access_token
	const testAccessJWT = "eyJ0eXAiOiAiSldUIiwgImFsZyI6ICJFZERTQSJ9.eyJpc3MiOiAic3RlYW0iLCAic3ViIjogIjc2NTYxMTk4MDEyMzQ1Njc4IiwgImF1ZCI6IFsid2ViOmNvbW11bml0eSIsICJ3ZWI6c3RvcmUiXSwgImV4cCI6IDIwMDAwMDAwMDAsICJuYmYiOiAxNzAwMDAwMDAwLCAiaWF0IjogMTcwMDAwMDAwMCwgImp0aSI6ICJ0ZXN0MTIzIiwgInBlciI6IDB9.ZmFrZXNpZzEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2"
	sess, err := auth.NewSession(
		auth.WithRefreshToken(testRefreshJWT),
		auth.WithAccessToken(testAccessJWT),
	)
	if err != nil {
		t.Fatalf("NewSession() error: %v", err)
	}
	client, err := NewClient(sess, "")
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}
	client.http = httpClient
	client.webAPIBaseURL = "https://api.test"

	_, err = client.GetOffer(context.Background(), 42, false)
	if err != nil {
		t.Fatalf("GetOffer() error: %v", err)
	}
}

func TestGetOffersRequiresDirection(t *testing.T) {
	client := &Client{apiKey: "api-key", http: http.DefaultClient}
	if _, err := client.GetOffers(context.Background(), GetOffersOptions{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestSendBuildsTradeOfferPayload(t *testing.T) {
	var form url.Values
	var referer string

	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/tradeoffer/new/send" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm() error: %v", err)
		}
		form = r.PostForm
		referer = r.Header.Get("Referer")
		return jsonResponse(`{"tradeofferid":"99"}`), nil
	})}

	client := newTestClient(t, httpClient)
	offerID, err := client.Send(context.Background(), SendOptions{
		PartnerSteamID: 76561198012345679,
		AccessToken:    "trade-token",
		MyItems: []Item{{
			AppID:     730,
			ContextID: 2,
			AssetID:   12345,
			Amount:    1,
		}},
		Message: "hello",
	})
	if err != nil {
		t.Fatalf("Send() error: %v", err)
	}
	if offerID != 99 {
		t.Fatalf("offerID = %d", offerID)
	}
	if got := form.Get("sessionid"); got != "session-123" {
		t.Errorf("sessionid = %q", got)
	}
	if got := form.Get("partner"); got != "76561198012345679" {
		t.Errorf("partner = %q", got)
	}
	if !strings.Contains(referer, "token=trade-token") {
		t.Errorf("referer = %q", referer)
	}

	var params map[string]string
	if err := json.Unmarshal([]byte(form.Get("trade_offer_create_params")), &params); err != nil {
		t.Fatalf("trade_offer_create_params decode: %v", err)
	}
	if params["trade_offer_access_token"] != "trade-token" {
		t.Errorf("trade token = %q", params["trade_offer_access_token"])
	}

	var tradeOffer struct {
		Me struct {
			Assets []Item `json:"assets"`
		} `json:"me"`
	}
	if err := json.Unmarshal([]byte(form.Get("json_tradeoffer")), &tradeOffer); err != nil {
		t.Fatalf("json_tradeoffer decode: %v", err)
	}
	if len(tradeOffer.Me.Assets) != 1 || tradeOffer.Me.Assets[0].AssetID != 12345 {
		t.Fatalf("assets = %#v", tradeOffer.Me.Assets)
	}
}

func TestAcceptReturnsSteamError(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(`{"strError":"nope"}`), nil
	})}

	client := newTestClient(t, httpClient)
	err := client.Accept(context.Background(), 123)
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*SteamError); !ok {
		t.Fatalf("error = %T %v", err, err)
	}
}

func newTestClient(t *testing.T, httpClient *http.Client) *Client {
	t.Helper()

	sess, err := auth.DeserializeSession(&auth.SerializedSession{
		RefreshToken: testRefreshJWT,
		SessionID:    "session-123",
	})
	if err != nil {
		t.Fatalf("DeserializeSession() error: %v", err)
	}
	client, err := NewClient(sess, "api-key")
	if err != nil {
		t.Fatalf("NewClient() error: %v", err)
	}
	client.http = httpClient
	client.webAPIBaseURL = "https://api.test"
	client.communityBaseURL = "https://community.test"
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
