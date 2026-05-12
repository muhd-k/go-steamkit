package trade

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
)

var (
	myEscrowPattern    = regexp.MustCompile(`(?i)g_daysMyEscrow[\s=]+(\d+);`)
	theirEscrowPattern = regexp.MustCompile(`(?i)g_daysTheirEscrow[\s=]+(\d+);`)
	notFriendsPattern  = regexp.MustCompile(`>You are not friends with this user<`)
)

// GetPartnerEscrowDuration returns the escrow hold duration for a potential trade partner.
func (c *Client) GetPartnerEscrowDuration(ctx context.Context, partnerSteamID uint64, accessToken string) (*EscrowDuration, error) {
	params := url.Values{
		"partner": {strconv.FormatUint(uint64(accountIDFromSteamID64(partnerSteamID)), 10)},
	}
	if accessToken != "" {
		params.Set("token", accessToken)
	}
	return c.getEscrowDuration(ctx, c.communityBaseURL+"/tradeoffer/new/?"+params.Encode())
}

// GetOfferEscrowDuration returns the escrow hold duration shown on an existing offer page.
func (c *Client) GetOfferEscrowDuration(ctx context.Context, offerID uint64) (*EscrowDuration, error) {
	return c.getEscrowDuration(ctx, c.communityBaseURL+"/tradeoffer/"+strconv.FormatUint(offerID, 10))
}

func (c *Client) getEscrowDuration(ctx context.Context, targetURL string) (*EscrowDuration, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("steamkit/trade: failed to create escrow request: %w", err)
	}
	for k, v := range steamapiHeaders() {
		req.Header[k] = v
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("steamkit/trade: failed to retrieve escrow duration: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("steamkit/trade: failed to read escrow response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("steamkit/trade: escrow HTTP %d: %s", resp.StatusCode, string(body))
	}
	return parseEscrowDuration(body)
}

func parseEscrowDuration(data []byte) (*EscrowDuration, error) {
	myMatch := myEscrowPattern.FindSubmatch(data)
	theirMatch := theirEscrowPattern.FindSubmatch(data)
	if myMatch == nil || theirMatch == nil {
		if notFriendsPattern.Match(data) {
			return nil, errors.New("steamkit/trade: you are not friends with this user")
		}
		return nil, errors.New("steamkit/trade: escrow duration markers not found")
	}

	myEscrow, err := strconv.ParseUint(string(myMatch[1]), 10, 32)
	if err != nil {
		return nil, fmt.Errorf("steamkit/trade: failed to parse own escrow duration: %w", err)
	}
	theirEscrow, err := strconv.ParseUint(string(theirMatch[1]), 10, 32)
	if err != nil {
		return nil, fmt.Errorf("steamkit/trade: failed to parse partner escrow duration: %w", err)
	}

	return &EscrowDuration{
		DaysMyEscrow:    uint32(myEscrow),
		DaysTheirEscrow: uint32(theirEscrow),
	}, nil
}
