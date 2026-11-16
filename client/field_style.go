package youtrack

import (
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

	errFieldStyleID = "field style id %q is not in the palette: it must be a number from %s to %d"
)

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
// one without an error. A style that carries a Background or a Foreground was read from YouTrack, which may
// have a larger palette than MaxFieldStyleID: it is accepted with any non-negative numeric ID.
type FieldStyle struct {
	ID         string `json:"id,omitempty"`
	Background string `json:"background,omitempty"`
	Foreground string `json:"foreground,omitempty"`
	Type       string `json:"$type,omitempty"`
}

// validateFieldStyle refuses a style whose ID is not in the palette. A nil style, which is not sent, is valid.
// The ID has to be written the way YouTrack returns it, as a number without sign or leading zeros. A style read
// from YouTrack, which carries its Background or Foreground, may have an ID above MaxFieldStyleID: it comes from
// a palette larger than the one the client knows, and sending it back must not fail.
func validateFieldStyle(style *FieldStyle) error {
	if style == nil {
		return nil
	}

	id, err := strconv.Atoi(style.ID)
	readFromYouTrack := style.Background != "" || style.Foreground != ""
	aboveKnownPalette := id > MaxFieldStyleID && !readFromYouTrack
	if err != nil || id < 0 || aboveKnownPalette || strconv.Itoa(id) != style.ID {
		return fmt.Errorf(errFieldStyleID, style.ID, DefaultFieldStyleID, MaxFieldStyleID)
	}

	return nil
}
