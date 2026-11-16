package youtrack

import (
	"fmt"
	"strconv"
	"testing"
	"time"
)

// TestIntegrationStateBundleColorPalette checks that a state bundle value is colored with the same palette as an
// enum bundle value: every style from DefaultFieldStyleID to MaxFieldStyleID comes back with its own ID, and the
// first ID above MaxFieldStyleID comes back as the default style.
func TestIntegrationStateBundleColorPalette(t *testing.T) {
	client, ctx := requireIntegrationConfig(t)

	values := make([]StateBundleElement, 0, MaxFieldStyleID+1)
	for id := range MaxFieldStyleID + 1 {
		values = append(values, StateBundleElement{
			Name:  fmt.Sprintf("style-%d", id),
			Color: &FieldStyle{ID: strconv.Itoa(id)},
		})
	}

	bundle, err := client.CreateStateBundle(ctx, StateBundle{
		Name:   fmt.Sprintf("client-state-palette-%d", time.Now().UnixNano()),
		Values: values,
	})
	if err != nil {
		t.Fatalf("failed to create state bundle: %v", err)
	}
	t.Cleanup(func() {
		if err := client.DeleteStateBundle(ctx, bundle.ID); err != nil {
			t.Errorf("failed to delete state bundle %s: %v", bundle.ID, err)
		}
	})

	read, err := client.GetStateBundleByID(ctx, bundle.ID)
	if err != nil {
		t.Fatalf("failed to read state bundle: %v", err)
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

	// addBundleValue skips the client-side check, to see what YouTrack does with an ID above the palette.
	above := StateBundleElement{
		Name:  "style-above-palette",
		Color: &FieldStyle{ID: strconv.Itoa(MaxFieldStyleID + 1)},
	}
	added, err := addBundleValue[StateBundleElement](ctx, client, stateBundleValues, bundle.ID, above)
	if err != nil {
		t.Fatalf("failed to add value above the palette: %v", err)
	}
	if added.Color == nil || added.Color.ID != DefaultFieldStyleID {
		t.Errorf("ID %d is now in the palette, raise MaxFieldStyleID: got color %+v", MaxFieldStyleID+1, added.Color)
	}
}
