package inventory

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

// Item is a concrete inventory instance (an asset linked to its description).
type Item struct {
	AppID      uint32
	ContextID  uint64
	AssetID    uint64
	ClassID    uint64
	InstanceID uint64
	Amount     uint64

	// Description is the class-level metadata for this item.
	Description *Description

	// Properties are raw asset properties (paint wear, float, etc.)
	// Only populated when GetInventoryOptions.IncludeProperties is true.
	Properties []*AssetProperty

	// Accessories are parent/child relationships (stickers, patches, charms, etc.)
	// Only populated when GetInventoryOptions.IncludeProperties is true.
	Accessories []*AssetAccessory
}

// Description contains Steam item metadata (class-level).
type Description struct {
	AppID      uint32 `json:"appid,string"`
	ClassID    uint64 `json:"classid,string"`
	InstanceID uint64 `json:"instanceid,string"`

	Name           string `json:"name"`
	MarketName     string `json:"market_name"`
	MarketHashName string `json:"market_hash_name"`

	IconURL         string `json:"icon_url"`
	IconURLLarge    string `json:"icon_url_large"`
	IconDragURL     string `json:"icon_drag_url"`

	NameColor       string `json:"name_color"`
	BackgroundColor string `json:"background_color"`
	Type            string `json:"type"`

	Tradable  bool `json:"tradable"`
	Marketable bool `json:"marketable"`
	Commodity  bool `json:"commodity"`

	MarketTradableRestriction  uint32 `json:"market_tradable_restriction,string"`
	MarketMarketableRestriction uint32 `json:"market_marketable_restriction,string"`

	Descriptions DescriptionLines `json:"descriptions"`
	Actions      []*Action        `json:"actions"`
	MarketActions []*Action       `json:"market_actions"`
	OwnerActions []*Action        `json:"owner_actions"`
	Tags         []*Tag           `json:"tags"`

	OwnerDescriptions DescriptionLines `json:"owner_descriptions"`
	FraudWarnings     []string         `json:"fraudwarnings"`

	Sealed bool `json:"sealed"`
}

// DescriptionLines is a slice of DescriptionLine that handles Steam's "" empty value.
type DescriptionLines []*DescriptionLine

func (d *DescriptionLines) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte(`""`)) {
		return nil
	}
	return json.Unmarshal(data, (*[]*DescriptionLine)(d))
}

// DescriptionLine is a single description entry.
type DescriptionLine struct {
	Value string  `json:"value"`
	Type  *string `json:"type"`
	Name  *string `json:"name"`
	Color *string `json:"color"`
}

// Action is a clickable action link for an item.
type Action struct {
	Name string `json:"name"`
	Link string `json:"link"`
}

// Tag is a classification tag for an item.
type Tag struct {
	InternalName     string `json:"internal_name"`
	Name             string `json:"name"`
	Category         string `json:"category"`
	CategoryName     string `json:"category_name"`
	Color            *string `json:"color"`
}

// AssetProperty is a raw property from raw_asset_properties (e.g. paint wear, float).
type AssetProperty struct {
	PropertyID uint64 `json:"propertyid"`
	Value      string `json:"value"`
}

// AssetAccessory represents a parent/child relationship (stickers, patches, etc.).
type AssetAccessory struct {
	ClassID                      uint64            `json:"classid"`
	ParentRelationshipProperties []*AssetProperty  `json:"parent_relationship_properties"`
	StandaloneProperties         []*AssetProperty  `json:"standalone_properties"`
}

// InventoryPage is a single page of inventory data returned by Steam.
type InventoryPage struct {
	// Items are the concrete inventory instances on this page.
	Items []*Item

	// TotalInventoryCount is the total number of items in this inventory.
	TotalInventoryCount int

	// LastAssetID is the last asset ID on this page, used for pagination.
	LastAssetID uint64

	// MoreAvailable is true if there are more pages to fetch.
	MoreAvailable bool
}

// --- internal JSON structs ---

// flexBool unmarshals from either a JSON boolean or number (0/1).
type flexBool bool

func (b *flexBool) UnmarshalJSON(data []byte) error {
	s := string(data)
	switch s {
	case "1", "true":
		*b = true
	case "0", "false":
		*b = false
	default:
		return fmt.Errorf("steamkit/inventory: invalid flexBool value: %s", s)
	}
	return nil
}

func (b flexBool) Bool() bool { return bool(b) }

// rawInventoryPage matches the JSON shape returned by Steam Community.
type rawInventoryPage struct {
	Success               flexBool            `json:"success"`
	Error                 string              `json:"error,omitempty"`
	TotalInventoryCount   int                 `json:"total_inventory_count"`
	Assets                []*rawAsset         `json:"assets"`
	Descriptions          []*rawDescription   `json:"descriptions"`
	LastAssetID           string              `json:"last_assetid"`
	MoreAvailable         int                 `json:"more_items"`
	AssetPropertiesData   []*rawAssetProperties `json:"asset_properties"`
}

type rawAsset struct {
	AppID      flexUint32 `json:"appid"`
	ContextID  flexUint64 `json:"contextid"`
	AssetID    flexUint64 `json:"assetid"`
	ClassID    flexUint64 `json:"classid"`
	InstanceID flexUint64 `json:"instanceid"`
	Amount     flexUint64 `json:"amount"`
	Pos        uint32     `json:"pos"`
}

type rawDescription struct {
	AppID      flexUint32 `json:"appid"`
	ClassID    flexUint64 `json:"classid"`
	InstanceID flexUint64 `json:"instanceid"`

	IconURL         string           `json:"icon_url"`
	IconURLLarge    string           `json:"icon_url_large"`
	IconDragURL     string           `json:"icon_drag_url"`
	Name            string           `json:"name"`
	MarketName      string           `json:"market_name"`
	MarketHashName  string           `json:"market_hash_name"`
	NameColor       string           `json:"name_color"`
	BackgroundColor string           `json:"background_color"`
	Type            string           `json:"type"`
	Tradable        uintBool         `json:"tradable"`
	Marketable      uintBool         `json:"marketable"`
	Commodity       uintBool         `json:"commodity"`
	MarketTradableRestriction  flexUint32 `json:"market_tradable_restriction"`
	MarketMarketableRestriction flexUint32 `json:"market_marketable_restriction"`
	Descriptions    DescriptionLines `json:"descriptions"`
	Actions         []*Action        `json:"actions"`
	MarketActions   []*Action        `json:"market_actions"`
	OwnerActions    []*Action        `json:"owner_actions"`
	Tags            []*Tag           `json:"tags"`
	OwnerDescriptions DescriptionLines `json:"owner_descriptions"`
	FraudWarnings   []string         `json:"fraudwarnings"`
	Sealed          uintBool         `json:"sealed"`
}

type rawAssetProperties struct {
	AssetID               flexUint64            `json:"assetid"`
	AssetProperties       []*rawAssetProperty   `json:"asset_properties"`
	AssetAccessories      []*rawAssetAccessory  `json:"asset_accessories"`
}

type rawAssetProperty struct {
	PropertyID uint64 `json:"propertyid"`
	Value      string `json:"value"`
}

type rawAssetAccessory struct {
	ClassID                      uint64             `json:"classid"`
	ParentRelationshipProperties []*rawAssetProperty `json:"parent_relationship_properties"`
	StandaloneProperties         []*rawAssetProperty `json:"standalone_properties"`
}

// uintBool handles Steam's 0/1 bools
type uintBool bool

func (b *uintBool) UnmarshalJSON(data []byte) error {
	s := string(data)
	switch s {
	case "1", "true":
		*b = true
	case "0", "false", "":
		*b = false
	default:
		return fmt.Errorf("steamkit/inventory: invalid uintBool value: %s", s)
	}
	return nil
}

// flexUint32 unmarshals from either a JSON string or number.
type flexUint32 uint32

func (f *flexUint32) UnmarshalJSON(data []byte) error {
	// Try unquoting if it's a string
	s := string(data)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	v, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return fmt.Errorf("steamkit/inventory: invalid flexUint32 value %q: %w", string(data), err)
	}
	*f = flexUint32(v)
	return nil
}

func (f flexUint32) Uint32() uint32 { return uint32(f) }

// flexUint64 unmarshals from either a JSON string or number.
type flexUint64 uint64

func (f *flexUint64) UnmarshalJSON(data []byte) error {
	s := string(data)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return fmt.Errorf("steamkit/inventory: invalid flexUint64 value %q: %w", string(data), err)
	}
	*f = flexUint64(v)
	return nil
}

func (f flexUint64) Uint64() uint64 { return uint64(f) }

func makeDescriptionKey(appID uint32, classID, instanceID uint64) string {
	return fmt.Sprintf("%d_%d_%d", appID, classID, instanceID)
}
