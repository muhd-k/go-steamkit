package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/skinflippa/go-steamkit/auth"
	"github.com/skinflippa/go-steamkit/internal/steamapi"
)

const defaultCommunityBaseURL = "https://steamcommunity.com"

// GetInventoryOptions controls how inventory is fetched.
type GetInventoryOptions struct {
	// Language is the language for item descriptions. Default is "english".
	Language string

	// Count is the maximum number of items per page. Default is 2000 (Steam max).
	Count int

	// IncludeProperties adds raw_asset_properties=1 to requests.
	// When true, Item.Properties and Item.Accessories are populated.
	IncludeProperties bool
}

func (o *GetInventoryOptions) language() string {
	if o == nil || o.Language == "" {
		return "english"
	}
	return o.Language
}

func (o *GetInventoryOptions) count() int {
	if o == nil || o.Count <= 0 {
		return 2000
	}
	return o.Count
}

func (o *GetInventoryOptions) includeProperties() bool {
	if o == nil {
		return false
	}
	return o.IncludeProperties
}

// Client fetches inventories from the Steam Community website.
type Client struct {
	session          *auth.SteamSession
	http             *http.Client
	communityBaseURL string
}

// NewClient creates an inventory client from an authenticated Steam session.
func NewClient(sess *auth.SteamSession) (*Client, error) {
	if sess == nil {
		return nil, ErrSessionRequired
	}
	if sess.APIClient() == nil {
		return nil, ErrSessionRequired
	}

	return &Client{
		session:          sess,
		http:             sess.APIClient().HTTPClient(),
		communityBaseURL: defaultCommunityBaseURL,
	}, nil
}

// GetAllInventory auto-paginates and returns every item in the requested inventory.
func (c *Client) GetAllInventory(ctx context.Context, steamID uint64, appID uint32, contextID uint64, opts *GetInventoryOptions) ([]*Item, error) {
	if err := c.requireCookies(); err != nil {
		return nil, err
	}

	var allItems []*Item
	var startAssetID string

	for {
		page, err := c.fetchPage(ctx, steamID, appID, contextID, startAssetID, opts)
		if err != nil {
			return nil, err
		}

		allItems = append(allItems, page.Items...)

		if !page.MoreAvailable {
			break
		}
		startAssetID = strconv.FormatUint(page.LastAssetID, 10)
	}

	return allItems, nil
}

// GetAllMyInventory is a convenience wrapper that fetches the current session's own inventory.
func (c *Client) GetAllMyInventory(ctx context.Context, appID uint32, contextID uint64, opts *GetInventoryOptions) ([]*Item, error) {
	steamID := c.session.SteamID64()
	if steamID == 0 {
		return nil, fmt.Errorf("steamkit/inventory: session has no SteamID")
	}
	return c.GetAllInventory(ctx, steamID, appID, contextID, opts)
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

func (c *Client) fetchPage(ctx context.Context, steamID uint64, appID uint32, contextID uint64, startAssetID string, opts *GetInventoryOptions) (*InventoryPage, error) {
	path := fmt.Sprintf("/inventory/%d/%d/%d", steamID, appID, contextID)
	params := url.Values{
		"l":     {opts.language()},
		"count": {strconv.Itoa(opts.count())},
	}
	if startAssetID != "" {
		params.Set("start_assetid", startAssetID)
	}
	if opts.includeProperties() {
		params.Set("raw_asset_properties", "1")
	}

	targetURL := c.communityBaseURL + path + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("steamkit/inventory: failed to create request: %w", err)
	}
	for k, v := range c.headers() {
		req.Header[k] = v
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("steamkit/inventory: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("steamkit/inventory: failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusForbidden {
		return nil, ErrInventoryPrivate
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrInventoryNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("steamkit/inventory: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var raw rawInventoryPage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("steamkit/inventory: failed to decode response: %w", err)
	}

	if !raw.Success.Bool() {
		if raw.Error != "" {
			return nil, newSteamErrorf("inventory error: %s", raw.Error)
		}
		return nil, ErrEmptyResponse
	}

	// Build description lookup map
	descMap := make(map[string]*rawDescription, len(raw.Descriptions))
	for _, d := range raw.Descriptions {
		if d == nil {
			continue
		}
		key := makeDescriptionKey(d.AppID.Uint32(), d.ClassID.Uint64(), d.InstanceID.Uint64())
		descMap[key] = d
	}

	// Build asset properties lookup map
	propMap := make(map[uint64]*rawAssetProperties)
	for _, p := range raw.AssetPropertiesData {
		if p == nil {
			continue
		}
		propMap[p.AssetID.Uint64()] = p
	}

	items := make([]*Item, 0, len(raw.Assets))
	for _, a := range raw.Assets {
		if a == nil {
			continue
		}
		item := &Item{
			AppID:      a.AppID.Uint32(),
			ContextID:  a.ContextID.Uint64(),
			AssetID:    a.AssetID.Uint64(),
			ClassID:    a.ClassID.Uint64(),
			InstanceID: a.InstanceID.Uint64(),
			Amount:     a.Amount.Uint64(),
		}

		// Link description
		key := makeDescriptionKey(item.AppID, item.ClassID, item.InstanceID)
		if rawDesc, ok := descMap[key]; ok {
			item.Description = rawDesc.toDescription()
		}

		// Link properties/accessories
		if rawProps, ok := propMap[item.AssetID]; ok {
			for _, rp := range rawProps.AssetProperties {
				if rp == nil {
					continue
				}
				item.Properties = append(item.Properties, &AssetProperty{
					PropertyID: rp.PropertyID,
					Value:      rp.Value,
				})
			}
			for _, ra := range rawProps.AssetAccessories {
				if ra == nil {
					continue
				}
				acc := &AssetAccessory{
					ClassID: ra.ClassID,
				}
				for _, rp := range ra.ParentRelationshipProperties {
					if rp == nil {
						continue
					}
					acc.ParentRelationshipProperties = append(acc.ParentRelationshipProperties, &AssetProperty{
						PropertyID: rp.PropertyID,
						Value:      rp.Value,
					})
				}
				for _, rp := range ra.StandaloneProperties {
					if rp == nil {
						continue
					}
					acc.StandaloneProperties = append(acc.StandaloneProperties, &AssetProperty{
						PropertyID: rp.PropertyID,
						Value:      rp.Value,
					})
				}
				item.Accessories = append(item.Accessories, acc)
			}
		}

		items = append(items, item)
	}

	lastAssetID, _ := strconv.ParseUint(raw.LastAssetID, 10, 64)

	return &InventoryPage{
		Items:               items,
		TotalInventoryCount: raw.TotalInventoryCount,
		LastAssetID:         lastAssetID,
		MoreAvailable:       raw.MoreAvailable != 0,
	}, nil
}

func (c *Client) headers() http.Header {
	h := http.Header{}
	h.Set("Accept", "application/json, text/plain, */*")
	h.Set("Referer", steamapi.CommunityURL+"/")
	h.Set("Origin", steamapi.CommunityURL)
	return h
}

func (d *rawDescription) toDescription() *Description {
	return &Description{
		AppID:                       d.AppID.Uint32(),
		ClassID:                     d.ClassID.Uint64(),
		InstanceID:                  d.InstanceID.Uint64(),
		IconURL:                     d.IconURL,
		IconURLLarge:                d.IconURLLarge,
		IconDragURL:                 d.IconDragURL,
		Name:                        d.Name,
		MarketName:                  d.MarketName,
		MarketHashName:              d.MarketHashName,
		NameColor:                   d.NameColor,
		BackgroundColor:             d.BackgroundColor,
		Type:                        d.Type,
		Tradable:                    bool(d.Tradable),
		Marketable:                  bool(d.Marketable),
		Commodity:                   bool(d.Commodity),
		MarketTradableRestriction:   d.MarketTradableRestriction.Uint32(),
		MarketMarketableRestriction: d.MarketMarketableRestriction.Uint32(),
		Descriptions:                d.Descriptions,
		Actions:                     d.Actions,
		MarketActions:               d.MarketActions,
		OwnerActions:                d.OwnerActions,
		Tags:                        d.Tags,
		OwnerDescriptions:           d.OwnerDescriptions,
		FraudWarnings:               d.FraudWarnings,
		Sealed:                      bool(d.Sealed),
	}
}
