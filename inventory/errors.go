package inventory

import (
	"errors"
	"fmt"
)

var (
	// ErrSessionRequired indicates that an inventory client needs an authenticated auth.SteamSession.
	ErrSessionRequired = errors.New("steamkit/inventory: authenticated session required")

	// ErrSessionCookiesRequired indicates that Steam Community cookies have not been obtained.
	ErrSessionCookiesRequired = errors.New("steamkit/inventory: session cookies required")

	// ErrInventoryPrivate indicates that the requested inventory is private.
	ErrInventoryPrivate = errors.New("steamkit/inventory: inventory is private")

	// ErrInventoryNotFound indicates that the requested inventory could not be found.
	ErrInventoryNotFound = errors.New("steamkit/inventory: inventory not found")

	// ErrEmptyResponse indicates that Steam returned a structurally valid response with no data.
	ErrEmptyResponse = errors.New("steamkit/inventory: Steam returned an empty response")
)

// SteamError is returned when Steam accepts a request but reports an action-level failure.
type SteamError struct {
	Message string
}

func (e *SteamError) Error() string {
	return fmt.Sprintf("steamkit/inventory: %s", e.Message)
}

func newSteamErrorf(format string, args ...interface{}) *SteamError {
	return &SteamError{Message: fmt.Sprintf(format, args...)}
}
