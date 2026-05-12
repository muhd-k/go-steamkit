package trade

import (
	"encoding/json"
	"strconv"
)

const steamID64IndividualBase = 76561197960265728

// State is the current lifecycle state of a Steam trade offer.
type State uint

const (
	StateInvalid                  State = 1
	StateActive                   State = 2
	StateAccepted                 State = 3
	StateCountered                State = 4
	StateExpired                  State = 5
	StateCanceled                 State = 6
	StateDeclined                 State = 7
	StateInvalidItems             State = 8
	StateCreatedNeedsConfirmation State = 9
	StateCanceledBySecondFactor   State = 10
	StateInEscrow                 State = 11
	StateReverted                 State = 12
)

// ConfirmationMethod describes the second-factor confirmation required for an offer.
type ConfirmationMethod uint

const (
	ConfirmationInvalid ConfirmationMethod = 0
	ConfirmationEmail   ConfirmationMethod = 1
	ConfirmationMobile  ConfirmationMethod = 2
)

// Asset is an item in a trade offer returned by Steam.
type Asset struct {
	AppID      uint32 `json:"appid"`
	ContextID  uint64 `json:"contextid,string"`
	AssetID    uint64 `json:"assetid,string"`
	CurrencyID uint64 `json:"currencyid,string"`
	ClassID    uint64 `json:"classid,string"`
	InstanceID uint64 `json:"instanceid,string"`
	Amount     uint64 `json:"amount,string"`
	Missing    bool   `json:"missing"`
}

// Item is an item included when sending a trade offer.
type Item struct {
	AppID      uint32 `json:"appid"`
	ContextID  uint64 `json:"contextid,string"`
	Amount     uint64 `json:"amount"`
	AssetID    uint64 `json:"assetid,string,omitempty"`
	CurrencyID uint64 `json:"currencyid,string,omitempty"`
}

// Description contains Steam item metadata returned with trade offers.
type Description struct {
	AppID           uint32 `json:"appid"`
	ClassID         uint64 `json:"classid,string"`
	InstanceID      uint64 `json:"instanceid,string"`
	IconURL         string `json:"icon_url"`
	IconURLLarge    string `json:"icon_url_large"`
	Name            string `json:"name"`
	MarketName      string `json:"market_name"`
	MarketHashName  string `json:"market_hash_name"`
	NameColor       string `json:"name_color"`
	BackgroundColor string `json:"background_color"`
	Type            string `json:"type"`
	Tradable        bool   `json:"tradable"`
	Commodity       bool   `json:"commodity"`
}

// Offer is a Steam trade offer.
type Offer struct {
	ID                 uint64             `json:"tradeofferid,string"`
	TradeID            uint64             `json:"tradeid,string"`
	OtherAccountID     uint32             `json:"accountid_other"`
	OtherSteamID       uint64             `json:"-"`
	Message            string             `json:"message"`
	ExpirationTime     uint32             `json:"expiration_time"`
	State              State              `json:"trade_offer_state"`
	ItemsToGive        []*Asset           `json:"items_to_give"`
	ItemsToReceive     []*Asset           `json:"items_to_receive"`
	IsOurOffer         bool               `json:"is_our_offer"`
	TimeCreated        uint32             `json:"time_created"`
	TimeUpdated        uint32             `json:"time_updated"`
	EscrowEndDate      uint32             `json:"escrow_end_date"`
	ConfirmationMethod ConfirmationMethod `json:"confirmation_method"`
}

func (o *Offer) UnmarshalJSON(data []byte) error {
	type alias Offer
	var v alias
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*o = Offer(v)
	if o.OtherAccountID != 0 {
		o.OtherSteamID = uint64(o.OtherAccountID) + steamID64IndividualBase
	}
	return nil
}

// Offers contains sent and received trade offers from IEconService/GetTradeOffers.
type Offers struct {
	Sent         []*Offer       `json:"trade_offers_sent"`
	Received     []*Offer       `json:"trade_offers_received"`
	Descriptions []*Description `json:"descriptions"`
}

// OfferResult contains a single offer and optional item descriptions.
type OfferResult struct {
	Offer        *Offer         `json:"offer"`
	Descriptions []*Description `json:"descriptions"`
}

// EscrowDuration is Steam's escrow hold duration for both trade parties.
type EscrowDuration struct {
	DaysMyEscrow    uint32
	DaysTheirEscrow uint32
}

func accountIDFromSteamID64(steamID uint64) uint32 {
	if steamID < steamID64IndividualBase {
		return uint32(steamID)
	}
	return uint32(steamID - steamID64IndividualBase)
}

func steamID64String(steamID uint64) string {
	return strconv.FormatUint(steamID, 10)
}
