package youtrack

import (
	"errors"
	"fmt"
)

// ErrNotFound is the common sentinel behind every "looked for it, it isn't there"
// error returned by this package. Lookups that scan a collection (by name, by login,
// by short name) wrap it, and IsNotFound also reports true for a 404 HTTP response,
// so callers can test for absence once instead of per-endpoint:
//
//	g, err := c.GetUserGroupByName(ctx, "onprem_elca-platform_admin")
//	switch {
//	case err == nil:
//	    // found
//	case youtrack.IsNotFound(err):
//	    // absent
//	default:
//	    // transport or server failure: unknown, retry
//	}
//
// Distinguishing absence from failure matters for reconciling callers: treating a
// transport error as absence leads to duplicate creates or deletion of live data.
var ErrNotFound = errors.New("not found")

// notFoundf builds an error that wraps ErrNotFound and reads naturally on its own,
// e.g. notFoundf("user with login %q", login) -> `user with login "jdoe": not found`.
func notFoundf(format string, args ...any) error {
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), ErrNotFound)
}

// entityNotFoundf builds an error that wraps a specific entity sentinel (which itself
// wraps ErrNotFound), so both the entity-specific predicate and IsNotFound match, while
// the message still reads identifier-first:
//
//	entityNotFoundf(errAppNotFound, "app with name %q", name)
//	  -> `app with name "Diagram Editor": app not found`
func entityNotFoundf(sentinel error, format string, args ...any) error {
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), sentinel)
}

// IsNotFound reports whether err indicates that the requested entity does not exist.
// It is true both for errors wrapping ErrNotFound (collection lookups that came up
// empty) and for a 404 HTTP response from the API.
//
// It is false for every other failure — timeouts, 5xx, unmarshalling errors — which
// mean the existence of the entity could not be determined.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound) || IsNotFoundError(err)
}
