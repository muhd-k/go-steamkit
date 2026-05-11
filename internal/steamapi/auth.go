package steamapi

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"

	"github.com/skinflippa/go-steamkit/internal/protowire"
)

// --- IAuthenticationService Wrappers ---
//
// Steam's IAuthenticationService requires protobuf-encoded request bodies
// sent via the `input_protobuf_encoded` form parameter (base64-encoded).
// Responses are also protobuf-encoded.
//
// We use our minimal protowire encoder/decoder to avoid the full protobuf dependency.
//
// API Reference: https://steamapi.xpaw.me/#IAuthenticationService

// RSAPublicKeyResponse contains the RSA public key for password encryption.
type RSAPublicKeyResponse struct {
	PublicKeyMod string
	PublicKeyExp string
	Timestamp    uint64
}

// GetPasswordRSAPublicKey fetches the RSA public key used to encrypt the account password.
//
// Steam API: GET IAuthenticationService/GetPasswordRSAPublicKey/v1
func (c *Client) GetPasswordRSAPublicKey(ctx context.Context, accountName string) (*RSAPublicKeyResponse, error) {
	enc := protowire.NewEncoder()
	enc.EncodeString(1, accountName)

	params := url.Values{
		"origin":                  {CommunityURL},
		"input_protobuf_encoded": {base64.StdEncoding.EncodeToString(enc.Bytes())},
	}

	respBytes, err := c.callProto(ctx, "GET", "IAuthenticationService/GetPasswordRSAPublicKey/v1", params)
	if err != nil {
		return nil, err
	}

	dec := protowire.NewDecoder(respBytes)
	result := &RSAPublicKeyResponse{}
	for !dec.Done() {
		fieldNum, wireType, err := dec.Field()
		if err != nil {
			break
		}
		switch fieldNum {
		case 1:
			result.PublicKeyMod, _ = dec.ReadString()
		case 2:
			result.PublicKeyExp, _ = dec.ReadString()
		case 3:
			result.Timestamp, _ = dec.ReadVarint()
		default:
			dec.Skip(wireType)
		}
	}

	return result, nil
}

// BeginAuthSessionResponse contains the result of starting an auth session.
type BeginAuthSessionResponse struct {
	ClientID              uint64
	RequestID             []byte
	Interval              float32
	AllowedConfirmations  []AllowedConfirmation
	SteamID               uint64
	WeakToken             string
	ExtendedErrorMessage  string
}

// AllowedConfirmation describes a confirmation method accepted by Steam.
type AllowedConfirmation struct {
	ConfirmationType  int
	AssociatedMessage string
}

// Guard type constants matching Steam's EAuthSessionGuardType enum.
const (
	GuardTypeUnknownValue             = 0
	GuardTypeNoneValue                = 1
	GuardTypeEmailCodeValue           = 2
	GuardTypeDeviceCodeValue          = 3
	GuardTypeDeviceConfirmationValue  = 4
	GuardTypeEmailConfirmationValue   = 5
)

// Steam's EAuthTokenPlatformType enum values.
const (
	PlatformTypeUnknown     = 0
	PlatformTypeSteamClient = 1
	PlatformTypeWebBrowser  = 2
	PlatformTypeMobileApp   = 3
)

// BeginAuthSessionViaCredentials starts the authentication flow.
//
// Steam API: POST IAuthenticationService/BeginAuthSessionViaCredentials/v1
func (c *Client) BeginAuthSessionViaCredentials(
	ctx context.Context,
	accountName string,
	encryptedPassword string,
	rsaTimestamp uint64,
	persistence bool,
	deviceFriendlyName string,
) (*BeginAuthSessionResponse, error) {
	// Build device_details sub-message (field 9)
	deviceEnc := protowire.NewEncoder()
	deviceEnc.EncodeString(1, deviceFriendlyName)
	deviceEnc.EncodeEnum(2, PlatformTypeWebBrowser)

	// Build main request message
	enc := protowire.NewEncoder()
	enc.EncodeString(1, deviceFriendlyName)
	enc.EncodeString(2, accountName)
	enc.EncodeString(3, encryptedPassword)
	enc.EncodeUint64(4, rsaTimestamp)
	enc.EncodeBool(5, persistence)
	enc.EncodeEnum(6, PlatformTypeWebBrowser)
	if persistence {
		enc.EncodeEnum(7, 1) // k_ESessionPersistence_Persistent
	}
	enc.EncodeString(8, "Community")
	enc.EncodeMessage(9, deviceEnc)
	// field 11 = language (0 = default, skipped)
	enc.EncodeInt32(12, 2) // qos_level

	params := url.Values{
		"input_protobuf_encoded": {base64.StdEncoding.EncodeToString(enc.Bytes())},
	}

	respBytes, err := c.callProto(ctx, "POST", "IAuthenticationService/BeginAuthSessionViaCredentials/v1", params)
	if err != nil {
		return nil, err
	}

	dec := protowire.NewDecoder(respBytes)
	result := &BeginAuthSessionResponse{}
	for !dec.Done() {
		fieldNum, wireType, err := dec.Field()
		if err != nil {
			break
		}
		switch fieldNum {
		case 1:
			result.ClientID, _ = dec.ReadVarint()
		case 2:
			result.RequestID, _ = dec.ReadBytes()
		case 3:
			v, _ := dec.ReadFixed32()
			result.Interval = math.Float32frombits(v)
		case 4:
			subData, _ := dec.ReadBytes()
			conf := decodeAllowedConfirmation(subData)
			result.AllowedConfirmations = append(result.AllowedConfirmations, conf)
		case 5:
			result.SteamID, _ = dec.ReadVarint()
		case 6:
			result.WeakToken, _ = dec.ReadString()
		case 8:
			result.ExtendedErrorMessage, _ = dec.ReadString()
		default:
			dec.Skip(wireType)
		}
	}

	return result, nil
}

func decodeAllowedConfirmation(data []byte) AllowedConfirmation {
	dec := protowire.NewDecoder(data)
	conf := AllowedConfirmation{}
	for !dec.Done() {
		fieldNum, wireType, err := dec.Field()
		if err != nil {
			break
		}
		switch fieldNum {
		case 1:
			v, _ := dec.ReadVarint()
			conf.ConfirmationType = int(v)
		case 2:
			conf.AssociatedMessage, _ = dec.ReadString()
		default:
			dec.Skip(wireType)
		}
	}
	return conf
}

// UpdateAuthSessionWithSteamGuardCode submits a Steam Guard code.
//
// Steam API: POST IAuthenticationService/UpdateAuthSessionWithSteamGuardCode/v1
func (c *Client) UpdateAuthSessionWithSteamGuardCode(
	ctx context.Context,
	clientID uint64,
	steamID uint64,
	code string,
	codeType int,
) error {
	enc := protowire.NewEncoder()
	enc.EncodeUint64(1, clientID)
	enc.EncodeFixed64(2, steamID) // steamid is fixed64
	enc.EncodeString(3, code)
	enc.EncodeEnum(4, codeType)

	params := url.Values{
		"input_protobuf_encoded": {base64.StdEncoding.EncodeToString(enc.Bytes())},
	}

	_, err := c.callProto(ctx, "POST", "IAuthenticationService/UpdateAuthSessionWithSteamGuardCode/v1", params)
	return err
}

// PollAuthSessionStatusResponse contains the result of polling the auth session.
type PollAuthSessionStatusResponse struct {
	NewClientID          uint64
	RefreshToken         string
	AccessToken          string
	HadRemoteInteraction bool
	AccountName          string
	NewGuardData         string
}

// PollAuthSessionStatus polls the current status of an auth session.
//
// Steam API: POST IAuthenticationService/PollAuthSessionStatus/v1
func (c *Client) PollAuthSessionStatus(
	ctx context.Context,
	clientID uint64,
	requestID []byte,
) (*PollAuthSessionStatusResponse, error) {
	enc := protowire.NewEncoder()
	enc.EncodeUint64(1, clientID)
	enc.EncodeBytes(2, requestID)

	params := url.Values{
		"input_protobuf_encoded": {base64.StdEncoding.EncodeToString(enc.Bytes())},
	}

	respBytes, err := c.callProto(ctx, "POST", "IAuthenticationService/PollAuthSessionStatus/v1", params)
	if err != nil {
		return nil, err
	}

	dec := protowire.NewDecoder(respBytes)
	result := &PollAuthSessionStatusResponse{}
	for !dec.Done() {
		fieldNum, wireType, err := dec.Field()
		if err != nil {
			break
		}
		switch fieldNum {
		case 1:
			result.NewClientID, _ = dec.ReadVarint()
		case 3:
			result.RefreshToken, _ = dec.ReadString()
		case 4:
			result.AccessToken, _ = dec.ReadString()
		case 5:
			v, _ := dec.ReadVarint()
			result.HadRemoteInteraction = v != 0
		case 6:
			result.AccountName, _ = dec.ReadString()
		case 7:
			result.NewGuardData, _ = dec.ReadString()
		default:
			dec.Skip(wireType)
		}
	}

	return result, nil
}

// GenerateAccessTokenResponse contains a newly generated access token.
type GenerateAccessTokenResponse struct {
	AccessToken  string
	RefreshToken string
}

// GenerateAccessTokenForApp requests a new access token using a refresh token.
//
// Steam API: POST IAuthenticationService/GenerateAccessTokenForApp/v1
func (c *Client) GenerateAccessTokenForApp(
	ctx context.Context,
	refreshToken string,
	steamID uint64,
) (*GenerateAccessTokenResponse, error) {
	enc := protowire.NewEncoder()
	enc.EncodeString(1, refreshToken)
	enc.EncodeFixed64(2, steamID) // steamid is fixed64

	params := url.Values{
		"input_protobuf_encoded": {base64.StdEncoding.EncodeToString(enc.Bytes())},
	}

	respBytes, err := c.callProto(ctx, "POST", "IAuthenticationService/GenerateAccessTokenForApp/v1", params)
	if err != nil {
		return nil, err
	}

	dec := protowire.NewDecoder(respBytes)
	result := &GenerateAccessTokenResponse{}
	for !dec.Done() {
		fieldNum, wireType, err := dec.Field()
		if err != nil {
			break
		}
		switch fieldNum {
		case 1:
			result.AccessToken, _ = dec.ReadString()
		case 2:
			result.RefreshToken, _ = dec.ReadString()
		default:
			dec.Skip(wireType)
		}
	}

	return result, nil
}

// --- Internal Proto Transport ---

// callProto makes a request to a Steam Web API protobuf endpoint.
func (c *Client) callProto(
	ctx context.Context,
	method string,
	endpoint string,
	params url.Values,
) ([]byte, error) {
	fullURL := WebAPIBaseURL + "/" + endpoint

	var req *http.Request
	var err error

	switch strings.ToUpper(method) {
	case "GET":
		if params == nil {
			params = url.Values{}
		}
		fullURL += "?" + params.Encode()
		req, err = http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	case "POST":
		if params == nil {
			params = url.Values{}
		}
		body := params.Encode()
		req, err = http.NewRequestWithContext(ctx, "POST", fullURL, strings.NewReader(body))
		if err == nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	default:
		return nil, fmt.Errorf("steamapi: unsupported HTTP method: %s", method)
	}

	if err != nil {
		return nil, fmt.Errorf("steamapi: failed to create request: %w", err)
	}

	for k, v := range apiHeaders() {
		req.Header[k] = v
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("steamapi: request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("steamapi: failed to read response body: %w", err)
	}

	// Check EResult header
	if eresult := resp.Header.Get("X-eresult"); eresult != "" && eresult != "1" {
		return nil, fmt.Errorf("steamapi: EResult %s, message: %s, body_len: %d",
			eresult,
			resp.Header.Get("X-error_message"),
			len(bodyBytes),
		)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("steamapi: HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return bodyBytes, nil
}
