// Package steamapi provides a low-level HTTP client for the Steam Web API.
//
// This is an internal package — consumers should use the auth, trade, and inventory
// packages instead. It handles request construction, protobuf/JSON encoding,
// EResult error checking, and cookie management.
package steamapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

const (
	// WebAPIBaseURL is the base URL for Steam Web API calls.
	WebAPIBaseURL = "https://api.steampowered.com"

	// CommunityURL is the Steam Community base URL.
	CommunityURL = "https://steamcommunity.com"

	// StoreURL is the Steam Store base URL.
	StoreURL = "https://store.steampowered.com"

	// LoginURL is the Steam login endpoint base URL.
	LoginURL = "https://login.steampowered.com"

	// HelpURL is the Steam Help base URL.
	HelpURL = "https://help.steampowered.com"

	// CheckoutURL is the Steam Checkout base URL.
	CheckoutURL = "https://checkout.steampowered.com"
)

// AllDomainURLs lists all Steam domains that receive auth cookies.
var AllDomainURLs = []string{
	CommunityURL,
	StoreURL,
	HelpURL,
	CheckoutURL,
}

// Client is a low-level HTTP client for the Steam Web API.
// It handles request formatting, EResult error checking, and cookie jar management.
type Client struct {
	httpClient *http.Client

	// AccessToken is set after successful authentication.
	// Used for authenticated API calls.
	AccessToken string
}

// NewClient creates a new Steam API client.
// If httpClient is nil, a default client with a cookie jar and IPv4-preferring transport is created.
func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		jar, _ := cookiejar.New(nil)
		// Use an IPv4-preferring dialer to avoid issues with IPv6 NAT64 addresses
		// on networks where IPv6 routes to Steam are unreachable.
		dialer := &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}
		transport := &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.DialContext(ctx, "tcp4", addr)
			},
			MaxIdleConns:        10,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		}
		httpClient = &http.Client{
			Jar:       jar,
			Transport: transport,
		}
	}
	if httpClient.Jar == nil {
		jar, _ := cookiejar.New(nil)
		httpClient.Jar = jar
	}
	return &Client{
		httpClient: httpClient,
	}
}

// HTTPClient returns the underlying *http.Client for direct use.
func (c *Client) HTTPClient() *http.Client {
	return c.httpClient
}

// apiHeaders returns the standard headers for Steam Web API requests.
func apiHeaders() http.Header {
	h := http.Header{}
	h.Set("Accept", "application/json, text/plain, */*")
	h.Set("Sec-Fetch-Dest", "empty")
	h.Set("Sec-Fetch-Mode", "cors")
	h.Set("Sec-Fetch-Site", "cross-site")
	h.Set("Referer", CommunityURL+"/")
	h.Set("Origin", CommunityURL)
	return h
}

// CallJSON makes a request to a Steam Web API endpoint and decodes the JSON response.
//
// The endpoint format is: IInterface/Method/vN
// For example: IAuthenticationService/GetPasswordRSAPublicKey/v1
//
// Query parameters and form data are merged into the request.
// For GET requests, all params go in the query string.
// For POST requests, params go as application/x-www-form-urlencoded body.
func (c *Client) CallJSON(
	ctx context.Context,
	method string,
	endpoint string,
	params url.Values,
	result interface{},
) error {
	fullURL := WebAPIBaseURL + "/" + endpoint

	var req *http.Request
	var err error

	switch strings.ToUpper(method) {
	case "GET":
		if params == nil {
			params = url.Values{}
		}
		params.Set("origin", CommunityURL)
		fullURL += "?" + params.Encode()
		req, err = http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	case "POST":
		if params == nil {
			params = url.Values{}
		}
		req, err = http.NewRequestWithContext(ctx, "POST", fullURL, strings.NewReader(params.Encode()))
		if err == nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	default:
		return fmt.Errorf("steamapi: unsupported HTTP method: %s", method)
	}

	if err != nil {
		return fmt.Errorf("steamapi: failed to create request: %w", err)
	}

	for k, v := range apiHeaders() {
		req.Header[k] = v
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("steamapi: request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check EResult header
	if eresult := resp.Header.Get("X-eresult"); eresult != "" && eresult != "1" {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("steamapi: EResult %s, message: %s, body: %s",
			eresult,
			resp.Header.Get("X-error_message"),
			string(bodyBytes),
		)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("steamapi: HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("steamapi: failed to decode response: %w", err)
		}
	}

	return nil
}

// PostForm makes a POST request with form-encoded data and returns the raw response.
// Used for non-API endpoints like steamcommunity.com tradeoffer operations.
func (c *Client) PostForm(ctx context.Context, targetURL string, data url.Values, headers http.Header) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("steamapi: failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	for k, v := range headers {
		req.Header[k] = v
	}

	return c.httpClient.Do(req)
}

// Get makes a GET request and returns the raw response.
func (c *Client) Get(ctx context.Context, targetURL string, headers http.Header) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("steamapi: failed to create request: %w", err)
	}

	for k, v := range headers {
		req.Header[k] = v
	}

	return c.httpClient.Do(req)
}

// SetCookie adds a cookie to the client's cookie jar for the given domain.
func (c *Client) SetCookie(domain, name, value string) {
	u, _ := url.Parse(domain)
	c.httpClient.Jar.SetCookies(u, []*http.Cookie{
		{Name: name, Value: value, Path: "/"},
	})
}

// GetCookies returns all cookies stored for the given domain.
func (c *Client) GetCookies(domain string) []*http.Cookie {
	u, _ := url.Parse(domain)
	return c.httpClient.Jar.Cookies(u)
}
