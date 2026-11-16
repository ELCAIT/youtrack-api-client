package youtrack

import (
	"fmt"
	"slices"
	"strconv"
	"testing"
	"time"
)

// TestIntegrationEnumBundleColorPalette checks that the palette of the instance is the one the client knows:
// every style from DefaultFieldStyleID to MaxFieldStyleID comes back with its own ID, and the first ID above
// MaxFieldStyleID comes back as the default style. A failure means YouTrack changed its palette and
// MaxFieldStyleID, the README table and the CHANGELOG have to follow it.
func TestIntegrationEnumBundleColorPalette(t *testing.T) {
	client, ctx := requireIntegrationConfig(t)

	values := make([]EnumBundleElement, 0, MaxFieldStyleID+1)
	for id := range MaxFieldStyleID + 1 {
		values = append(values, EnumBundleElement{
			Name:  fmt.Sprintf("style-%d", id),
			Color: &FieldStyle{ID: strconv.Itoa(id)},
		})
	}

	bundle, err := client.CreateEnumBundle(ctx, EnumBundle{
		Name:   fmt.Sprintf("client-palette-%d", time.Now().UnixNano()),
		Values: values,
	})
	if err != nil {
		t.Fatalf("failed to create enum bundle: %v", err)
	}
	t.Cleanup(func() {
		if err := client.DeleteEnumBundle(ctx, bundle.ID); err != nil {
			t.Errorf("failed to delete enum bundle %s: %v", bundle.ID, err)
		}
	})

	read, err := client.GetEnumBundleByID(ctx, bundle.ID)
	if err != nil {
		t.Fatalf("failed to read enum bundle: %v", err)
	}

	if len(read.Values) != MaxFieldStyleID+1 {
		t.Fatalf("expected %d values in the bundle read back, got %d", MaxFieldStyleID+1, len(read.Values))
	}

	for _, value := range read.Values {
		var want string
		if _, err := fmt.Sscanf(value.Name, "style-%s", &want); err != nil {
			t.Fatalf("unexpected value name %q: %v", value.Name, err)
		}
		if value.Color == nil || value.Color.ID != want {
			t.Errorf("value %s: got color %+v, want style %s", value.Name, value.Color, want)
		}
	}

	// A value read from YouTrack is sent back as it is, with its Background, Foreground and $type.
	lastName := fmt.Sprintf("style-%d", MaxFieldStyleID)
	index := slices.IndexFunc(read.Values, func(value EnumBundleElement) bool { return value.Name == lastName })
	if index < 0 {
		t.Fatalf("value %s not found in the bundle read back", lastName)
	}
	roundTrip := read.Values[index]
	replaced, err := client.ReplaceEnumBundleValue(ctx, bundle.ID, roundTrip.ID, EnumBundleValueUpdate{
		Name: roundTrip.Name, Color: roundTrip.Color,
	})
	if err != nil {
		t.Fatalf("failed to send back the value read from YouTrack: %v", err)
	}
	if replaced.Color == nil || *replaced.Color != *roundTrip.Color {
		t.Errorf("color changed on the round trip: got %+v, want %+v", replaced.Color, roundTrip.Color)
	}

	// addBundleValue skips the client-side check, to see what YouTrack does with an ID above the palette.
	above := EnumBundleElement{
		Name:  "style-above-palette",
		Color: &FieldStyle{ID: strconv.Itoa(MaxFieldStyleID + 1)},
	}
	added, err := addBundleValue[EnumBundleElement](ctx, client, enumBundleValues, bundle.ID, above)
	if err != nil {
		t.Fatalf("failed to add value above the palette: %v", err)
	}
	if added.Color == nil || added.Color.ID != DefaultFieldStyleID {
		t.Errorf("ID %d is now in the palette, raise MaxFieldStyleID: got color %+v", MaxFieldStyleID+1, added.Color)
	}
}
