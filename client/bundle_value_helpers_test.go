package youtrack

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

const (
	testBundleID      = "66-1"
	testBundleValueID = "67-1"
	testStyleID       = "20"
	testStyleBackdrop = "#DB3B4B"
	testStyleInk      = "#fff"
	testValueName     = "Critical"
	keyColor          = "color"
	keyID             = "id"
)

// TestEnumAndStateBundleValueRequests checks the per-value calls address the
// right endpoint and that a replace sends null for what it clears.
func TestEnumAndStateBundleValueRequests(t *testing.T) {
	t.Parallel()

	enumValues := "/" + enumBundlesAPIPath + "/" + testBundleID + "/values"
	stateValues := "/" + stateBundlesAPIPath + "/" + testBundleID + "/values"
	description := "Blocks a release"

	tests := []struct {
		name     string
		wantPath string
		wantBody map[string]any
		call     func(context.Context, *Client) error
	}{
		{
			name:     "add enum value",
			wantPath: enumValues,
			wantBody: map[string]any{"name": "Critical"},
			call: func(ctx context.Context, c *Client) error {
				_, err := c.AddEnumBundleValue(ctx, testBundleID, EnumBundleElement{Name: "Critical"})
				return err
			},
		},
		{
			name:     "replace enum value clears localized name and description",
			wantPath: enumValues + "/" + testBundleValueID,
			wantBody: map[string]any{"name": "Critical", "localizedName": nil, "description": nil, "archived": false},
			call: func(ctx context.Context, c *Client) error {
				_, err := c.ReplaceEnumBundleValue(ctx, testBundleID, testBundleValueID, EnumBundleValueUpdate{Name: "Critical"})
				return err
			},
		},
		{
			name:     "add enum value with a color sends the style id",
			wantPath: enumValues,
			wantBody: map[string]any{"name": testValueName, keyColor: map[string]any{keyID: testStyleID}},
			call: func(ctx context.Context, c *Client) error {
				_, err := c.AddEnumBundleValue(ctx, testBundleID, EnumBundleElement{
					Name: testValueName, Color: &FieldStyle{ID: testStyleID},
				})
				return err
			},
		},
		{
			name:     "replace enum value with a color sends the style id",
			wantPath: enumValues + "/" + testBundleValueID,
			wantBody: map[string]any{
				"name": testValueName, "localizedName": nil, "description": nil, "archived": false,
				keyColor: map[string]any{keyID: testStyleID},
			},
			call: func(ctx context.Context, c *Client) error {
				_, err := c.ReplaceEnumBundleValue(ctx, testBundleID, testBundleValueID, EnumBundleValueUpdate{
					Name: testValueName, Color: &FieldStyle{ID: testStyleID},
				})
				return err
			},
		},
		{
			name:     "replace enum value resets the color with the default style",
			wantPath: enumValues + "/" + testBundleValueID,
			wantBody: map[string]any{
				"name": testValueName, "localizedName": nil, "description": nil, "archived": false,
				keyColor: map[string]any{keyID: DefaultFieldStyleID},
			},
			call: func(ctx context.Context, c *Client) error {
				_, err := c.ReplaceEnumBundleValue(ctx, testBundleID, testBundleValueID, EnumBundleValueUpdate{
					Name: testValueName, Color: &FieldStyle{ID: DefaultFieldStyleID},
				})
				return err
			},
		},
		{
			name:     "add state value",
			wantPath: stateValues,
			wantBody: map[string]any{"name": "Done", "isResolved": true},
			call: func(ctx context.Context, c *Client) error {
				_, err := c.AddStateBundleValue(ctx, testBundleID, StateBundleElement{Name: "Done", IsResolved: true})
				return err
			},
		},
		{
			name:     "replace state value sends every field",
			wantPath: stateValues + "/" + testBundleValueID,
			wantBody: map[string]any{
				"name": "Done", "localizedName": nil, "description": description,
				"isResolved": false, "archived": true, "ordinal": float64(2),
			},
			call: func(ctx context.Context, c *Client) error {
				ordinal := 2
				_, err := c.ReplaceStateBundleValue(ctx, testBundleID, testBundleValueID, StateBundleValueUpdate{
					Name: "Done", Description: &description, Archived: true, Ordinal: &ordinal,
				})
				return err
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf(errUnexpectedMethod, r.Method)
				}
				if r.URL.Path != tc.wantPath {
					t.Errorf(fmtUnexpectedPath, r.URL.Path)
				}
				assertJSONBody(t, r, tc.wantBody)
				encodeJSON(t, w, map[string]string{"id": testBundleValueID})
			})
			defer server.Close()

			if err := tc.call(context.Background(), client); err != nil {
				t.Fatalf(fmtUnexpectedError, err)
			}
		})
	}
}

// TestEnumBundleValueColorIsRequestedAndDecoded checks that every call returning enum values asks YouTrack
// for the color, which it only returns when it is named in the fields, and decodes it. A value the server
// reports without a color is decoded with a nil Color.
func TestEnumBundleValueColorIsRequestedAndDecoded(t *testing.T) {
	t.Parallel()

	colored := map[string]any{
		keyID: testBundleValueID, "name": testValueName,
		keyColor: map[string]any{
			keyID: testStyleID, "background": testStyleBackdrop, "foreground": testStyleInk, "$type": "FieldStyle",
		},
	}
	plain := map[string]any{keyID: "67-2", "name": "Minor"}
	bundle := map[string]any{keyID: testBundleID, "values": []any{colored, plain}}

	tests := []struct {
		name       string
		wantFields string
		response   any
		call       func(context.Context, *Client) ([]EnumBundleElement, error)
	}{
		{
			name:       "add enum value",
			wantFields: enumBundleValueFields,
			response:   colored,
			call: func(ctx context.Context, c *Client) ([]EnumBundleElement, error) {
				value, err := c.AddEnumBundleValue(ctx, testBundleID, EnumBundleElement{Name: testValueName})
				if err != nil {
					return nil, err
				}
				return []EnumBundleElement{*value}, nil
			},
		},
		{
			name:       "replace enum value",
			wantFields: enumBundleValueFields,
			response:   colored,
			call: func(ctx context.Context, c *Client) ([]EnumBundleElement, error) {
				value, err := c.ReplaceEnumBundleValue(
					ctx, testBundleID, testBundleValueID, EnumBundleValueUpdate{Name: testValueName})
				if err != nil {
					return nil, err
				}
				return []EnumBundleElement{*value}, nil
			},
		},
		{
			name:       "get enum bundle",
			wantFields: strings.TrimPrefix(enumBundleFieldsParam, "fields="),
			response:   bundle,
			call: func(ctx context.Context, c *Client) ([]EnumBundleElement, error) {
				got, err := c.GetEnumBundleByID(ctx, testBundleID)
				if err != nil {
					return nil, err
				}
				return got.Values, nil
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.Query().Get("fields"); got != tc.wantFields {
					t.Errorf("unexpected fields: got %s, want %s", got, tc.wantFields)
				}
				encodeJSON(t, w, tc.response)
			})
			defer server.Close()

			values, err := tc.call(context.Background(), client)
			if err != nil {
				t.Fatalf(fmtUnexpectedError, err)
			}

			want := FieldStyle{
				ID: testStyleID, Background: testStyleBackdrop, Foreground: testStyleInk, Type: "FieldStyle",
			}
			if values[0].Color == nil || *values[0].Color != want {
				t.Errorf("unexpected color: got %+v, want %+v", values[0].Color, want)
			}
			if len(values) > 1 && values[1].Color != nil {
				t.Errorf("a value without color must decode a nil Color, got %+v", values[1].Color)
			}
		})
	}
}

// TestValidateFieldStyle covers the IDs of the palette and the ones that are not.
func TestValidateFieldStyle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		style   *FieldStyle
		wantErr bool
	}{
		{name: "no style is not sent", style: nil},
		{name: "default style", style: &FieldStyle{ID: DefaultFieldStyleID}},
		{name: "style of the palette", style: &FieldStyle{ID: testStyleID}},
		{name: "last style of the palette", style: &FieldStyle{ID: strconv.Itoa(MaxFieldStyleID)}},
		{name: "style with a background and a foreground", style: &FieldStyle{
			ID: testStyleID, Background: testStyleBackdrop, Foreground: testStyleInk, Type: "FieldStyle",
		}},
		{name: "first ID above the palette", style: &FieldStyle{ID: strconv.Itoa(MaxFieldStyleID + 1)}, wantErr: true},
		{name: "ID above the palette with a background and a foreground", style: &FieldStyle{
			ID: strconv.Itoa(MaxFieldStyleID + 1), Background: testStyleBackdrop, Foreground: testStyleInk,
		}, wantErr: true},
		{name: "negative ID", style: &FieldStyle{ID: "-1"}, wantErr: true},
		{name: "no ID", style: &FieldStyle{Background: testStyleBackdrop, Foreground: testStyleInk}, wantErr: true},
		{name: "ID that is not a number", style: &FieldStyle{ID: "red"}, wantErr: true},
		{name: "ID with a leading zero", style: &FieldStyle{ID: "05"}, wantErr: true},
		{name: "ID with a sign", style: &FieldStyle{ID: "+5"}, wantErr: true},
		{name: "ID with a fraction", style: &FieldStyle{ID: "1.5"}, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validateFieldStyle(tc.style)
			if tc.wantErr != (err != nil) {
				t.Fatalf("unexpected result: wantErr %v, got %v", tc.wantErr, err)
			}
			if tc.wantErr && !IsInvalidFieldStyleError(err) {
				t.Errorf("the error is not an invalid field style error: %v", err)
			}
		})
	}
}

// TestEnumBundleWritesRefuseAStyleOutsideThePalette checks that a style YouTrack would silently replace with
// the default one is refused before any request is sent.
func TestEnumBundleWritesRefuseAStyleOutsideThePalette(t *testing.T) {
	t.Parallel()

	outside := &FieldStyle{ID: strconv.Itoa(MaxFieldStyleID + 1)}

	tests := []struct {
		name string
		call func(context.Context, *Client) error
	}{
		{
			name: "add enum value",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.AddEnumBundleValue(ctx, testBundleID, EnumBundleElement{Name: testValueName, Color: outside})
				return err
			},
		},
		{
			name: "replace enum value",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.ReplaceEnumBundleValue(
					ctx, testBundleID, testBundleValueID, EnumBundleValueUpdate{Name: testValueName, Color: outside})
				return err
			},
		},
		{
			name: "create enum bundle",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.CreateEnumBundle(ctx, EnumBundle{Name: "Priority", Values: []EnumBundleElement{
					{Name: "Low", Color: &FieldStyle{ID: testStyleID}},
					{Name: testValueName, Color: outside},
				}})
				return err
			},
		},
		{
			name: "update enum bundle with a new value",
			call: func(ctx context.Context, c *Client) error {
				_, err := c.UpdateEnumBundle(ctx, testBundleID, EnumBundle{Values: []EnumBundleElement{
					{ID: testBundleValueID, Name: "Low", Color: outside},
					{Name: testValueName, Color: outside},
				}})
				return err
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, func(_ http.ResponseWriter, r *http.Request) {
				t.Errorf("a request was sent: %s %s", r.Method, r.URL.Path)
			})
			defer server.Close()

			err := tc.call(context.Background(), client)
			if err == nil {
				t.Fatal(errExpectedError)
			}
			if !IsInvalidFieldStyleError(err) {
				t.Errorf("the error is not an invalid field style error: %v", err)
			}
		})
	}
}

// bundleValueDeleteHandler serves the value delete, then the bundle reads that
// follow it: the first read still lists the value, as YouTrack does right after
// acknowledging the delete, and later reads no longer do.
func bundleValueDeleteHandler(t *testing.T, apiPath string, listed, gone any, reads *atomic.Int32) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodDelete:
			if r.URL.Path != "/"+apiPath+"/"+testBundleID+"/values/"+testBundleValueID {
				t.Errorf(fmtUnexpectedPath, r.URL.Path)
			}
		case http.MethodGet:
			if reads.Add(1) == 1 {
				encodeJSON(t, w, listed)
				return
			}
			encodeJSON(t, w, gone)
		default:
			t.Errorf(errUnexpectedMethod, r.Method)
		}
	}
}

// TestEnumAndStateBundleValueDeleteWaitsUntilGone covers YouTrack
// acknowledging a value delete before applying it.
func TestEnumAndStateBundleValueDeleteWaitsUntilGone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		apiPath string
		listed  any
		gone    any
		remove  func(context.Context, *Client) error
	}{
		{
			name:    "enum",
			apiPath: enumBundlesAPIPath,
			listed:  EnumBundle{ID: testBundleID, Values: []EnumBundleElement{{ID: testBundleValueID}}},
			gone:    EnumBundle{ID: testBundleID},
			remove: func(ctx context.Context, c *Client) error {
				return c.DeleteEnumBundleValue(ctx, testBundleID, testBundleValueID)
			},
		},
		{
			name:    "state",
			apiPath: stateBundlesAPIPath,
			listed:  StateBundle{ID: testBundleID, Values: []StateBundleElement{{ID: testBundleValueID}}},
			gone:    StateBundle{ID: testBundleID},
			remove: func(ctx context.Context, c *Client) error {
				return c.DeleteStateBundleValue(ctx, testBundleID, testBundleValueID)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var reads atomic.Int32
			client, server := newTestClient(t, bundleValueDeleteHandler(t, tc.apiPath, tc.listed, tc.gone, &reads))
			defer server.Close()

			if err := tc.remove(context.Background(), client); err != nil {
				t.Fatalf(fmtUnexpectedError, err)
			}
			if got := reads.Load(); got != 2 {
				t.Fatalf("expected 2 reads before the value was gone, got %d", got)
			}
		})
	}
}

// sequencedDeleteHandler answers the nth delete with the nth status, repeating
// the last one once they run out, and sends body with every non-OK status.
func sequencedDeleteHandler(t *testing.T, responses []int, body string, calls *atomic.Int32) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf(errUnexpectedMethod, r.Method)
		}
		status := responses[len(responses)-1]
		if n := int(calls.Add(1)); n <= len(responses) {
			status = responses[n-1]
		}
		w.WriteHeader(status)
		if status != http.StatusOK {
			_, _ = w.Write([]byte(body))
		}
	}
}

// TestDeleteBundleRetriesWhileInUse covers YouTrack refusing to delete a bundle
// until it has dropped the reference from a field removed just before.
func TestDeleteBundleRetriesWhileInUse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		responses []int
		body      string
		wantCalls int32
		wantErr   bool
	}{
		{name: "in use then deleted", responses: []int{http.StatusBadRequest, http.StatusOK}, body: `{"error":"x","error_description":"This bundle has usages you are not allowed to delete it"}`, wantCalls: 2},
		{name: "other bad request is not retried", responses: []int{http.StatusBadRequest}, body: `{"error":"x","error_description":"Something else"}`, wantCalls: 1, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32
			client, server := newTestClient(t, sequencedDeleteHandler(t, tc.responses, tc.body, &calls))
			defer server.Close()

			err := client.DeleteOwnedBundle(context.Background(), testBundleID)
			checkErr(t, err, tc.wantErr)
			if got := calls.Load(); got != tc.wantCalls {
				t.Fatalf("expected %d delete calls, got %d", tc.wantCalls, got)
			}
		})
	}
}
