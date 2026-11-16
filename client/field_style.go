package youtrack

import (
	"errors"
	"fmt"
	"strconv"
)

const (
	// fieldStyleFields is the fields selector for the color of a bundle value.
	fieldStyleFields = "color(id,background,foreground,$type)"

	// DefaultFieldStyleID is the ID of the style YouTrack gives a bundle value that has no color of its own.
	DefaultFieldStyleID = "0"

	// MaxFieldStyleID is the highest ID of the palette of YouTrack 2026.2, which has 36 styles, from
	// DefaultFieldStyleID to this one. The palette is listed in the README.
	MaxFieldStyleID = 35

	errFieldStyleID = "%w: id %q is not in the palette, it must be a number from %s to %d"
)

var errInvalidFieldStyle = errors.New("invalid field style")

// FieldStyle is the color of a bundle value.
//
// YouTrack offers a fixed palette of styles: a value is colored by picking one by its ID, and the Background
// and Foreground colors are those of the style. Verified against YouTrack 2026.2:
//   - a style sent without an ID is refused with "Incorrect style id provided", so a custom Background and
//     Foreground cannot be set;
//   - when an ID is sent next to a Background and a Foreground, the ID wins;
//   - an ID that is not in the palette is not refused: the value gets the default style, DefaultFieldStyleID;
//   - the color cannot be sent as null: to reset a value, send DefaultFieldStyleID.
//
// A write of a value that carries a style whose ID is not a number from DefaultFieldStyleID to
// MaxFieldStyleID is refused before it is sent, rather than left to YouTrack to replace the ID with the default
// one without an error, see ValidateFieldStyleID. A style read from an instance with a larger palette, which
// has an ID above MaxFieldStyleID, is refused too until MaxFieldStyleID follows the palette.
type FieldStyle struct {
	ID         string `json:"id,omitempty"`
	Background string `json:"background,omitempty"`
	Foreground string `json:"foreground,omitempty"`
	Type       string `json:"$type,omitempty"`
}

// IsInvalidFieldStyleError checks whether an error indicates that a style was refused before being sent because
// its ID is not in the palette. The failure is permanent: sending the same style again fails the same way.
func IsInvalidFieldStyleError(err error) bool {
	return errors.Is(err, errInvalidFieldStyle)
}

// ValidateFieldStyleID refuses a style ID that is not in the palette: a number from DefaultFieldStyleID to
// MaxFieldStyleID, written the way YouTrack returns it, without sign or leading zeros. It lets a caller check a
// configured ID before any write, and the error is reported by IsInvalidFieldStyleError.
func ValidateFieldStyleID(id string) error {
	number, err := strconv.Atoi(id)
	if err != nil || number < 0 || number > MaxFieldStyleID || strconv.Itoa(number) != id {
		return fmt.Errorf(errFieldStyleID, errInvalidFieldStyle, id, DefaultFieldStyleID, MaxFieldStyleID)
	}

	return nil
}

// validateFieldStyle refuses a style whose ID is not in the palette. A nil style, which is not sent, is valid.
func validateFieldStyle(style *FieldStyle) error {
	if style == nil {
		return nil
	}

	return ValidateFieldStyleID(style.ID)
}
