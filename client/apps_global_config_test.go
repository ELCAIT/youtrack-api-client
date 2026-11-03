package youtrack

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

const testSettingsDoc = `{"youtrackUrl":"http://youtrack:8080","apiToken":"<***>"}`

func TestGetAppGlobalConfig(t *testing.T) {
	t.Parallel()

	client, server := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != httpMethodGet {
			t.Errorf(fmtUnexpectedMethod, r.Method, httpMethodGet)
		}
		if r.URL.Path != testAppPath {
			t.Errorf(fmtUnexpectedPath, r.URL.Path)
		}
		encodeJSON(t, w, map[string]any{"globalConfig": AppGlobalConfig{
			Enabled:        true,
			GlobalSettings: testSettingsDoc,
			VisibilitySettings: &AppVisibilitySettings{
				PermittedGroups: []AppVisibilityGroup{{ID: "6-0", Name: "Registered Users"}},
				PermittedUsers:  []AppVisibilityUser{},
			},
		}})
	})
	defer server.Close()

	got, err := client.GetAppGlobalConfig(context.Background(), testAppID)
	if checkErr(t, err, false) {
		return
	}
	if !got.Enabled || got.GlobalSettings != testSettingsDoc {
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

	settings := testSettingsDoc
	enabled := false

	tests := []struct {
		name   string
		update AppGlobalConfigUpdate
		want   string
	}{
		{name: "settings only", update: AppGlobalConfigUpdate{GlobalSettings: &settings}, want: `{"globalSettings":` + mustJSON(t, settings) + `}`},
		{name: "enabled only", update: AppGlobalConfigUpdate{Enabled: &enabled}, want: `{"enabled":false}`},
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
				encodeJSON(t, w, AppGlobalConfig{Enabled: enabled, GlobalSettings: settings})
			})
			defer server.Close()

			got, err := client.SetAppGlobalConfig(context.Background(), testAppID, tc.update)
			if checkErr(t, err, false) {
				return
			}
			if got.GlobalSettings != settings {
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
				encodeJSON(t, w, map[string]any{"globalConfig": AppGlobalConfig{
					VisibilitySettings: &AppVisibilitySettings{PermittedGroups: []AppVisibilityGroup{{ID: "6-0"}}},
				}})
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
