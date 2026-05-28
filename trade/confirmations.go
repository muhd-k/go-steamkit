package trade

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	confirmationBasePath = "/mobileconf"
)

// ConfirmationType identifies the kind of pending mobile confirmation.
type ConfirmationType int

const (
	ConfirmationTypeUnknown             ConfirmationType = -1
	ConfirmationTypeInvalid             ConfirmationType = 0
	ConfirmationTypeTest                ConfirmationType = 1
	ConfirmationTypeTrade               ConfirmationType = 2
	ConfirmationTypeMarketListing       ConfirmationType = 3
	ConfirmationTypeFeatureOptOut       ConfirmationType = 4
	ConfirmationTypePhoneNumberChange   ConfirmationType = 5
	ConfirmationTypeAccountRecovery     ConfirmationType = 6
	ConfirmationTypeBuildChangeRequest  ConfirmationType = 7
	ConfirmationTypeAddUser             ConfirmationType = 8
	ConfirmationTypeRegisterAPIKey      ConfirmationType = 9
	ConfirmationTypeInviteToFamilyGroup ConfirmationType = 10
	ConfirmationTypeJoinFamilyGroup     ConfirmationType = 11
	ConfirmationTypeMarketPurchase      ConfirmationType = 12
	ConfirmationTypeRequestRefund       ConfirmationType = 13
)

// Confirmation is a pending Steam mobile confirmation.
type Confirmation struct {
	ID           uint64
	Nonce        string
	CreatorID    uint64
	CreationTime time.Time
	Type         ConfirmationType
	Accept       string
	Cancel       string
	Icon         string
	Multi        bool
	Headline     string
	Summary      []string
	Warn         string
}

type confirmationListResponse struct {
	Success  bool   `json:"success"`
	NeedAuth bool   `json:"needauth"`
	Message  string `json:"message"`
	Conf     []struct {
		ID           string          `json:"id"`
		Nonce        string          `json:"nonce"`
		CreatorID    string          `json:"creator_id"`
		CreationTime int64           `json:"creation_time"`
		Type         int             `json:"type"`
		Accept       string          `json:"accept"`
		Cancel       string          `json:"cancel"`
		Icon         string          `json:"icon"`
		Multi        bool            `json:"multi"`
		Headline     string          `json:"headline"`
		Summary      []string        `json:"summary"`
		Warn         json.RawMessage `json:"warn"` // Steam returns string or []string
	} `json:"conf"`
}

type confirmationActionResponse struct {
	Success  bool   `json:"success"`
	NeedAuth bool   `json:"needauth"`
	Message  string `json:"message"`
}

// PendingConfirmations returns all currently pending mobile confirmations.
func (c *Client) PendingConfirmations(ctx context.Context, identitySecret string) ([]*Confirmation, error) {
	return c.PendingConfirmationsWithDevice(ctx, identitySecret, "")
}

// PendingConfirmationsWithDevice returns pending confirmations using a specific mobile device id.
func (c *Client) PendingConfirmationsWithDevice(ctx context.Context, identitySecret, deviceID string) ([]*Confirmation, error) {
	params, err := c.confirmationParams(identitySecret, deviceID, "getlist", time.Now())
	if err != nil {
		return nil, err
	}

	// Fetch raw body first for logging/debugging
	rawBody, err := c.getRawBody(ctx, c.communityBaseURL+confirmationBasePath+"/getlist", params)
	if err != nil {
		return nil, err
	}

	// Save raw response to disk for analysis
	_ = os.WriteFile("steam_confirmations_raw.json", rawBody, 0600)

	var out confirmationListResponse
	if err := json.Unmarshal(rawBody, &out); err != nil {
		return nil, fmt.Errorf("steamkit/trade: failed to decode response: %w", err)
	}
	if err := checkConfirmationResponse(out.Success, out.NeedAuth, out.Message); err != nil {
		return nil, err
	}

	confs := make([]*Confirmation, 0, len(out.Conf))
	for _, raw := range out.Conf {
		id, err := strconv.ParseUint(raw.ID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("steamkit/trade: failed to parse confirmation id %q: %w", raw.ID, err)
		}
		creatorID, err := strconv.ParseUint(raw.CreatorID, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("steamkit/trade: failed to parse confirmation creator id %q: %w", raw.CreatorID, err)
		}
		// warn can be a string or []string in the Steam response
		warnStr := decodeWarn(raw.Warn)
		confs = append(confs, &Confirmation{
			ID:           id,
			Nonce:        raw.Nonce,
			CreatorID:    creatorID,
			CreationTime: time.Unix(raw.CreationTime, 0),
			Type:         parseConfirmationType(raw.Type),
			Accept:       raw.Accept,
			Cancel:       raw.Cancel,
			Icon:         raw.Icon,
			Multi:        raw.Multi,
			Headline:     raw.Headline,
			Summary:      raw.Summary,
			Warn:         warnStr,
		})
	}
	return confs, nil
}

// ConfirmOffer accepts the pending mobile confirmation for a sent or countered trade offer.
func (c *Client) ConfirmOffer(ctx context.Context, offerID uint64, identitySecret string) (*Confirmation, error) {
	return c.ConfirmOfferWithDevice(ctx, offerID, identitySecret, "")
}

// ConfirmOfferWithDevice accepts the pending trade confirmation using a specific mobile device id.
func (c *Client) ConfirmOfferWithDevice(ctx context.Context, offerID uint64, identitySecret, deviceID string) (*Confirmation, error) {
	if deviceID == "" {
		deviceID = generateDeviceID()
	}
	conf, err := c.FindConfirmation(ctx, offerID, identitySecret, deviceID)
	if err != nil {
		return nil, err
	}
	if err := c.AcceptConfirmationWithDevice(ctx, conf, identitySecret, deviceID); err != nil {
		return nil, err
	}
	return conf, nil
}

// FindConfirmation returns the pending confirmation with the requested creator id.
func (c *Client) FindConfirmation(ctx context.Context, creatorID uint64, identitySecret, deviceID string) (*Confirmation, error) {
	confs, err := c.PendingConfirmationsWithDevice(ctx, identitySecret, deviceID)
	if err != nil {
		return nil, err
	}
	for _, conf := range confs {
		if conf.CreatorID == creatorID {
			return conf, nil
		}
	}
	return nil, ErrConfirmationNotFound
}

// AcceptConfirmation accepts a pending mobile confirmation.
func (c *Client) AcceptConfirmation(ctx context.Context, conf *Confirmation, identitySecret string) error {
	return c.AcceptConfirmationWithDevice(ctx, conf, identitySecret, "")
}

// AcceptConfirmationWithDevice accepts a pending confirmation using a specific mobile device id.
func (c *Client) AcceptConfirmationWithDevice(ctx context.Context, conf *Confirmation, identitySecret, deviceID string) error {
	return c.sendConfirmationAction(ctx, conf, identitySecret, deviceID, true)
}

// DenyConfirmation denies a pending mobile confirmation.
func (c *Client) DenyConfirmation(ctx context.Context, conf *Confirmation, identitySecret string) error {
	return c.DenyConfirmationWithDevice(ctx, conf, identitySecret, "")
}

// DenyConfirmationWithDevice denies a pending confirmation using a specific mobile device id.
func (c *Client) DenyConfirmationWithDevice(ctx context.Context, conf *Confirmation, identitySecret, deviceID string) error {
	return c.sendConfirmationAction(ctx, conf, identitySecret, deviceID, false)
}

func (c *Client) sendConfirmationAction(ctx context.Context, conf *Confirmation, identitySecret, deviceID string, accept bool) error {
	if conf == nil {
		return ErrConfirmationNotFound
	}

	op := "cancel"
	if accept {
		op = "allow"
	}
	// Always use Nonce as ck — this matches aiosteampy's proven approach.
	// The separate Accept/Cancel fields may be empty in practice.
	nonce := conf.Nonce

	params, err := c.confirmationParams(identitySecret, deviceID, op, time.Now())
	if err != nil {
		return err
	}
	params.Set("op", op)
	params.Set("cid", strconv.FormatUint(conf.ID, 10))
	params.Set("ck", nonce)

	// Fetch raw response for logging
	rawBody, err := c.getRawBody(ctx, c.communityBaseURL+confirmationBasePath+"/ajaxop", params)
	if err != nil {
		return err
	}
	_ = os.WriteFile("steam_accept_raw.json", rawBody, 0600)

	var out confirmationActionResponse
	if err := json.Unmarshal(rawBody, &out); err != nil {
		return fmt.Errorf("steamkit/trade: failed to decode accept response: %w", err)
	}
	return checkConfirmationResponse(out.Success, out.NeedAuth, out.Message)
}

func (c *Client) confirmationParams(identitySecret, deviceID, tag string, now time.Time) (url.Values, error) {
	if err := c.requireCookies(); err != nil {
		return nil, err
	}
	if c.session.SteamID64() == 0 {
		return nil, ErrSessionRequired
	}
	if deviceID == "" {
		deviceID = generateDeviceID()
	}

	key, timestamp, err := generateConfirmationKey(identitySecret, tag, now)
	if err != nil {
		return nil, err
	}

	return url.Values{
		"p":   {deviceID},
		"a":   {c.session.SteamID()},
		"k":   {key},
		"t":   {strconv.FormatInt(timestamp, 10)},
		"m":   {"react"},
		"tag": {tag},
	}, nil
}

func generateConfirmationKey(identitySecret, tag string, now time.Time) (string, int64, error) {
	secret, err := base64.StdEncoding.DecodeString(identitySecret)
	if err != nil {
		return "", 0, fmt.Errorf("steamkit/trade: invalid identity secret: %w", err)
	}
	timestamp := now.Unix()

	var timeBytes [8]byte
	binary.BigEndian.PutUint64(timeBytes[:], uint64(timestamp))

	mac := hmac.New(sha1.New, secret)
	_, _ = mac.Write(timeBytes[:])
	_, _ = mac.Write([]byte(tag))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil)), timestamp, nil
}

func generateDeviceID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "android:00000000-0000-4000-8000-000000000000"
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	hexed := hex.EncodeToString(b[:])
	return "android:" + hexed[0:8] + "-" + hexed[8:12] + "-" + hexed[12:16] + "-" + hexed[16:20] + "-" + hexed[20:32]
}

func parseConfirmationType(v int) ConfirmationType {
	if v < int(ConfirmationTypeInvalid) || v > int(ConfirmationTypeRequestRefund) {
		return ConfirmationTypeUnknown
	}
	return ConfirmationType(v)
}

// decodeWarn handles Steam returning warn as either a JSON string or a JSON array.
func decodeWarn(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Try string first
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// Fall back to array of strings
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return strings.Join(arr, "\n")
	}
	return ""
}

func checkConfirmationResponse(success bool, needAuth bool, message string) error {
	if needAuth {
		return ErrUnauthenticated
	}
	if success {
		return nil
	}
	if message != "" {
		return newSteamErrorf("confirmation error: %s", message)
	}
	return newSteamErrorf("confirmation error: Steam returned success=false")
}

func (c *Client) getConfirmationDetails(ctx context.Context, confID uint64, identitySecret, deviceID string) (string, error) {
	params, err := c.confirmationParams(identitySecret, deviceID, "details"+strconv.FormatUint(confID, 10), time.Now())
	if err != nil {
		return "", err
	}
	var out struct {
		Success  bool   `json:"success"`
		NeedAuth bool   `json:"needauth"`
		Message  string `json:"message"`
		HTML     string `json:"html"`
	}
	if err := c.getJSON(ctx, c.communityBaseURL+confirmationBasePath+"/details/"+strconv.FormatUint(confID, 10), params, &out); err != nil {
		return "", err
	}
	if err := checkConfirmationResponse(out.Success, out.NeedAuth, out.Message); err != nil {
		return "", err
	}
	return out.HTML, nil
}
