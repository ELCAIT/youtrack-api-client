package youtrack

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

const (
	testSettingsDoc = `{"youtrackUrl":"http://youtrack:8080","apiToken":"<***>"}`

	// testSettingsWrite has no character json.Marshal escapes, so the payload
	// can be compared byte for byte.
	testSettingsWrite = `{"youtrackUrl":"http://youtrack:8080","apiToken":"perm-x"}`
)

// testWireConfig is a global config as YouTrack answers it, with the settings
// wrapped in a JSON string.
func testWireConfig(enabled bool, settings string, visibility *AppVisibilitySettings) appGlobalConfigWire {
	return appGlobalConfigWire{
		AppGlobalConfig: AppGlobalConfig{Enabled: enabled, VisibilitySettings: visibility},
		GlobalSettings:  settings,
	}
}

func TestGetAppGlobalConfig(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != httpMethodGet {
			t.Errorf(fmtUnexpectedMethod, r.Method, httpMethodGet)
		}
		if r.URL.Path != testAppPath {
			t.Errorf(fmtUnexpectedPath, r.URL.Path)
		}
		encodeJSON(t, w, map[string]any{"globalConfig": testWireConfig(true, testSettingsDoc, &AppVisibilitySettings{
			PermittedGroups: []AppVisibilityGroup{{ID: "6-0", Name: "Registered Users"}},
			PermittedUsers:  []AppVisibilityUser{},
		})})
	})
	defer server.Close()

	got, err := client.GetAppGlobalConfig(context.Background(), testAppID)
	if checkErr(t, err, false) {
		return
	}
	if !got.Enabled || string(got.GlobalSettings) != testSettingsDoc {
		t.Fatalf("unexpected config: %+v", got)
	}
	if len(got.VisibilitySettings.PermittedGroups) != 1 || got.VisibilitySettings.PermittedGroups[0].ID != "6-0" {
		t.Fatalf("unexpected visibility: %+v", got.VisibilitySettings)
	}
}

func TestGetAppGlobalConfigMissing(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		encodeJSON(t, w, map[string]any{})
	})
	defer server.Close()

	_, err := client.GetAppGlobalConfig(context.Background(), testAppID)
	checkErr(t, err, true)
}

func TestSetAppGlobalConfig(t *testing.T) {
	t.Parallel()

	enabled := false

	tests := []struct {
		name   string
		update AppGlobalConfigUpdate
		want   string
	}{
		// the settings go out as a JSON string holding the object, not as the object
		{name: "raw settings", update: AppGlobalConfigUpdate{GlobalSettings: json.RawMessage(testSettingsWrite)}, want: `{"globalSettings":` + mustJSON(t, testSettingsWrite) + `}`},
		{name: "map settings", update: AppGlobalConfigUpdate{GlobalSettings: map[string]any{"a": 1}}, want: `{"globalSettings":"{\"a\":1}"}`},
		{name: "enabled only", update: AppGlobalConfigUpdate{Enabled: &enabled}, want: `{"enabled":false}`},
		{name: "empty raw settings are left alone", update: AppGlobalConfigUpdate{Enabled: &enabled, GlobalSettings: json.RawMessage(nil)}, want: `{"enabled":false}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != httpMethodPost {
					t.Errorf(fmtUnexpectedMethod, r.Method, httpMethodPost)
				}
				if r.URL.Path != testAppPath+"/globalConfig" {
					t.Errorf(fmtUnexpectedPath, r.URL.Path)
				}
				body, _ := io.ReadAll(r.Body)
				if string(body) != tc.want {
					t.Errorf("unexpected payload: got %s, want %s", body, tc.want)
				}
				encodeJSON(t, w, testWireConfig(enabled, testSettingsDoc, nil))
			})
			defer server.Close()

			got, err := client.SetAppGlobalConfig(context.Background(), testAppID, tc.update)
			if checkErr(t, err, false) {
				return
			}
			if string(got.GlobalSettings) != testSettingsDoc {
				t.Fatalf("unexpected settings: %s", got.GlobalSettings)
			}
		})
	}
}

func TestSetAppVisibility(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		groups []string
		users  []string
		want   string
	}{
		{name: "groups and users", groups: []string{"6-0"}, users: []string{"1-1"}, want: `{"permittedGroups":[{"id":"6-0"}],"permittedUsers":[{"id":"1-1"}]}`},
		// nil must go out as [], not null, or YouTrack leaves the list as it was.
		{name: "nobody", want: `{"permittedGroups":[],"permittedUsers":[]}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != httpMethodPost {
					t.Errorf(fmtUnexpectedMethod, r.Method, httpMethodPost)
				}
				if r.URL.Path != testAppPath {
					t.Errorf(fmtUnexpectedPath, r.URL.Path)
				}
				body, _ := io.ReadAll(r.Body)
				if !strings.Contains(string(body), tc.want) {
					t.Errorf("unexpected payload: got %s, want it to contain %s", body, tc.want)
				}
				encodeJSON(t, w, map[string]any{"globalConfig": testWireConfig(true, "", &AppVisibilitySettings{
					PermittedGroups: []AppVisibilityGroup{{ID: "6-0"}},
				})})
			})
			defer server.Close()

			got, err := client.SetAppVisibility(context.Background(), testAppID, tc.groups, tc.users)
			if checkErr(t, err, false) {
				return
			}
			if len(got.PermittedGroups) != 1 {
				t.Fatalf("unexpected visibility: %+v", got)
			}
		})
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()

	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}

	return string(b)
}
