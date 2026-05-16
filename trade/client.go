package trade

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/muhd-k/go-steamkit/auth"
	"github.com/muhd-k/go-steamkit/internal/steamapi"
)

const (
	defaultWebAPIBaseURL    = "https://api.steampowered.com"
	defaultCommunityBaseURL = "https://steamcommunity.com"
)

// Client manages Steam trade offers for an authenticated account.
type Client struct {
	session *auth.SteamSession
	apiKey  string
	http    *http.Client

	webAPIBaseURL    string
	communityBaseURL string
}

// NewClient creates a trade client from an authenticated Steam session.
//
// The apiKey parameter is optional. If empty, the client will use the
// session's access token for IEconService Web API calls instead.
//
// Call sess.ObtainCookies or sess.FinalizeLoginViaWeb before using methods that
// touch steamcommunity.com, such as Send or Accept.
func NewClient(sess *auth.SteamSession, apiKey string) (*Client, error) {
	if sess == nil {
		return nil, ErrSessionRequired
	}
	if sess.APIClient() == nil {
		return nil, ErrSessionRequired
	}

	return &Client{
		session:          sess,
		apiKey:           apiKey,
		http:             sess.APIClient().HTTPClient(),
		webAPIBaseURL:    defaultWebAPIBaseURL,
		communityBaseURL: defaultCommunityBaseURL,
	}, nil
}

// authParams returns url.Values containing either "key" (if an API key was
// provided) or "access_token" (fallback to the session's access token).
func (c *Client) authParams() url.Values {
	if c.apiKey != "" {
		return url.Values{"key": {c.apiKey}}
	}
	if c.session != nil && c.session.AccessToken() != nil {
		return url.Values{"access_token": {c.session.AccessToken().Raw}}
	}
	return url.Values{}
}

// GetOffer retrieves a single trade offer by ID.
func (c *Client) GetOffer(ctx context.Context, offerID uint64, descriptions bool) (*OfferResult, error) {
	params := c.authParams()
	params.Set("tradeofferid", strconv.FormatUint(offerID, 10))
	params.Set("language", "en_us")
	if descriptions {
		params.Set("get_descriptions", "1")
	}

	var out struct {
		Response *OfferResult `json:"response"`
	}
	if err := c.getJSON(ctx, c.webAPI("IEconService/GetTradeOffer/v1"), params, &out); err != nil {
		return nil, err
	}
	if out.Response == nil || out.Response.Offer == nil {
		return nil, ErrEmptyResponse
	}
	return out.Response, nil
}

// GetOffersOptions controls which offers are returned by GetOffers.
type GetOffersOptions struct {
	Sent                 bool
	Received             bool
	Descriptions         bool
	ActiveOnly           bool
	HistoricalOnly       bool
	TimeHistoricalCutoff *uint32
}

// GetOffers retrieves sent and/or received trade offers.
func (c *Client) GetOffers(ctx context.Context, opts GetOffersOptions) (*Offers, error) {
	if !opts.Sent && !opts.Received {
		return nil, fmt.Errorf("steamkit/trade: Sent and Received cannot both be false")
	}

	params := c.authParams()
	if opts.Sent {
		params.Set("get_sent_offers", "1")
	}
	if opts.Received {
		params.Set("get_received_offers", "1")
	}
	if opts.Descriptions {
		params.Set("get_descriptions", "1")
		params.Set("language", "en_us")
	}
	if opts.ActiveOnly {
		params.Set("active_only", "1")
	}
	if opts.HistoricalOnly {
		params.Set("historical_only", "1")
	}
	if opts.TimeHistoricalCutoff != nil {
		params.Set("time_historical_cutoff", strconv.FormatUint(uint64(*opts.TimeHistoricalCutoff), 10))
	}

	var out struct {
		Response *Offers `json:"response"`
	}
	if err := c.getJSON(ctx, c.webAPI("IEconService/GetTradeOffers/v1"), params, &out); err != nil {
		return nil, err
	}
	if out.Response == nil {
		return nil, ErrEmptyResponse
	}
	return out.Response, nil
}

// GetActiveOffers retrieves currently active sent and received offers.
func (c *Client) GetActiveOffers(ctx context.Context, descriptions bool) (*Offers, error) {
	return c.GetOffers(ctx, GetOffersOptions{
		Sent:         true,
		Received:     true,
		Descriptions: descriptions,
		ActiveOnly:   true,
	})
}

// Decline declines a received trade offer.
func (c *Client) Decline(ctx context.Context, offerID uint64) error {
	return c.tradeAction(ctx, "IEconService/DeclineTradeOffer/v1", offerID)
}

// Cancel cancels a sent trade offer.
func (c *Client) Cancel(ctx context.Context, offerID uint64) error {
	return c.tradeAction(ctx, "IEconService/CancelTradeOffer/v1", offerID)
}

func (c *Client) tradeAction(ctx context.Context, endpoint string, offerID uint64) error {
	data := c.authParams()
	data.Set("tradeofferid", strconv.FormatUint(offerID, 10))
	return c.postForm(ctx, c.webAPI(endpoint), data, nil, nil)
}

// Accept accepts a received trade offer.
func (c *Client) Accept(ctx context.Context, offerID uint64) error {
	if err := c.requireCookies(); err != nil {
		return err
	}

	baseURL := fmt.Sprintf("%s/tradeoffer/%d/", c.communityBaseURL, offerID)
	data := url.Values{
		"sessionid":    {c.session.SessionID()},
		"serverid":     {"1"},
		"tradeofferid": {strconv.FormatUint(offerID, 10)},
	}
	headers := http.Header{"Referer": {baseURL}}

	var out actionResponse
	return c.postForm(ctx, baseURL+"accept", data, headers, &out)
}

// SendOptions describes a new trade offer.
type SendOptions struct {
	PartnerSteamID   uint64
	AccessToken      string
	MyItems          []Item
	TheirItems       []Item
	CounteredOfferID *uint64
	Message          string
}

// Send sends a new trade offer and returns the created offer ID.
func (c *Client) Send(ctx context.Context, opts SendOptions) (uint64, error) {
	if err := c.requireCookies(); err != nil {
		return 0, err
	}
	if len(opts.MyItems) == 0 && len(opts.TheirItems) == 0 {
		return 0, ErrEmptyTradeOffer
	}
	if opts.PartnerSteamID == 0 {
		return 0, fmt.Errorf("steamkit/trade: partner SteamID is required")
	}

	tradeOffer := map[string]interface{}{
		"newversion": true,
		"version":    len(opts.MyItems) + len(opts.TheirItems) + 1,
		"me": map[string]interface{}{
			"assets":   opts.MyItems,
			"currency": []struct{}{},
			"ready":    false,
		},
		"them": map[string]interface{}{
			"assets":   opts.TheirItems,
			"currency": []struct{}{},
			"ready":    false,
		},
	}

	tradeJSON, err := json.Marshal(tradeOffer)
	if err != nil {
		return 0, fmt.Errorf("steamkit/trade: failed to encode trade offer: %w", err)
	}

	data := url.Values{
		"sessionid":         {c.session.SessionID()},
		"serverid":          {"1"},
		"partner":           {steamID64String(opts.PartnerSteamID)},
		"tradeoffermessage": {opts.Message},
		"json_tradeoffer":   {string(tradeJSON)},
		"captcha":           {""},
	}

	var referer string
	if opts.CounteredOfferID != nil {
		data.Set("tradeofferid_countered", strconv.FormatUint(*opts.CounteredOfferID, 10))
		referer = fmt.Sprintf("%s/tradeoffer/%d/", c.communityBaseURL, *opts.CounteredOfferID)
	} else {
		createParams := map[string]string{}
		referer = fmt.Sprintf("%s/tradeoffer/new/?partner=%d", c.communityBaseURL, accountIDFromSteamID64(opts.PartnerSteamID))
		if opts.AccessToken != "" {
			createParams["trade_offer_access_token"] = opts.AccessToken
			referer += "&token=" + url.QueryEscape(opts.AccessToken)
		}
		paramsJSON, err := json.Marshal(createParams)
		if err != nil {
			return 0, fmt.Errorf("steamkit/trade: failed to encode trade offer params: %w", err)
		}
		data.Set("trade_offer_create_params", string(paramsJSON))
	}

	headers := http.Header{"Referer": {referer}}
	var out struct {
		actionResponse
		TradeOfferID uint64 `json:"tradeofferid,string"`
	}
	if err := c.postForm(ctx, c.communityBaseURL+"/tradeoffer/new/send", data, headers, &out); err != nil {
		return 0, err
	}
	if out.TradeOfferID == 0 {
		return 0, newSteamErrorf("send error: Steam returned empty tradeofferid")
	}
	return out.TradeOfferID, nil
}

type actionResponse struct {
	StrError string `json:"strError"`
}

func (c *Client) requireCookies() error {
	if c.session == nil {
		return ErrSessionRequired
	}
	if c.session.SessionID() == "" {
		return ErrSessionCookiesRequired
	}
	return nil
}

func (c *Client) webAPI(endpoint string) string {
	return c.webAPIBaseURL + "/" + endpoint
}

func (c *Client) getJSON(ctx context.Context, targetURL string, params url.Values, out interface{}) error {
	if len(params) > 0 {
		targetURL += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return fmt.Errorf("steamkit/trade: failed to create request: %w", err)
	}
	for k, v := range steamapiHeaders() {
		req.Header[k] = v
	}
	return c.doJSON(req, out)
}

func (c *Client) postForm(ctx context.Context, targetURL string, data url.Values, headers http.Header, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return fmt.Errorf("steamkit/trade: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for k, v := range steamapiHeaders() {
		req.Header[k] = v
	}
	for k, v := range headers {
		req.Header[k] = v
	}
	return c.doJSON(req, out)
}

func (c *Client) doJSON(req *http.Request, out interface{}) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("steamkit/trade: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("steamkit/trade: failed to read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("steamkit/trade: HTTP %d: %s", resp.StatusCode, string(body))
	}
	if len(body) == 0 {
		if out == nil {
			return nil
		}
		return ErrEmptyResponse
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("steamkit/trade: failed to decode response: %w", err)
	}
	if r, ok := out.(*actionResponse); ok && r.StrError != "" {
		return newSteamErrorf("%s", r.StrError)
	}
	if r, ok := responseWithSteamError(out); ok && r != "" {
		return newSteamErrorf("%s", r)
	}
	return nil
}

func responseWithSteamError(out interface{}) (string, bool) {
	b, err := json.Marshal(out)
	if err != nil {
		return "", false
	}
	var r actionResponse
	if err := json.Unmarshal(b, &r); err != nil {
		return "", false
	}
	return r.StrError, true
}

func steamapiHeaders() http.Header {
	h := http.Header{}
	h.Set("Accept", "application/json, text/plain, */*")
	h.Set("Referer", steamapi.CommunityURL+"/")
	h.Set("Origin", steamapi.CommunityURL)
	return h
}
