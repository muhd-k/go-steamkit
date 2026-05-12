package trade

import (
	"errors"
	"fmt"
)

var (
	// ErrSessionRequired indicates that a trade client needs an authenticated auth.SteamSession.
	ErrSessionRequired = errors.New("steamkit/trade: authenticated session required")

	// ErrSessionCookiesRequired indicates that Steam Community cookies have not been obtained.
	ErrSessionCookiesRequired = errors.New("steamkit/trade: session cookies required")

	// ErrAPIKeyRequired indicates that an IEconService Web API key is required.
	ErrAPIKeyRequired = errors.New("steamkit/trade: API key required")

	// ErrEmptyTradeOffer indicates that a send request had no items on either side.
	ErrEmptyTradeOffer = errors.New("steamkit/trade: trade offer must include at least one item")

	// ErrEmptyResponse indicates that Steam returned a structurally valid response with no data.
	ErrEmptyResponse = errors.New("steamkit/trade: Steam returned an empty response")
)

// SteamError is returned when Steam accepts a request but reports an action-level failure.
type SteamError struct {
	Message string
}

func (e *SteamError) Error() string {
	return fmt.Sprintf("steamkit/trade: %s", e.Message)
}

func newSteamErrorf(format string, args ...interface{}) *SteamError {
	return &SteamError{Message: fmt.Sprintf(format, args...)}
}
