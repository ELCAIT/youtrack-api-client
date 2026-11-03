package youtrack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// The global configuration of an app uses the same undocumented endpoints as
// the rest of apps.go; see the note there. Verified against YouTrack 2026.2
// (build 19656).
//
// An app declares its own settings in settings.json, so the settings are
// exchanged as the JSON document YouTrack stores, not as a typed struct. Two
// behaviours callers must plan for:
//   - Settings declared with `"format": "secret"` are never returned: a read
//     answers "<***>" in their place. A caller that manages such a setting has
//     to keep the value it wrote, and must not write the masked value back.
//   - On a read, entity settings (a user group, say) come back expanded with
//     every field YouTrack knows (`$type`, `ringId`, `name`, ...), while a
//     write needs only the fields the UI sends (`id`, `key`, `label`,
//     `checkbox`). A read therefore never compares equal to what was written.

const (
	appGlobalConfigSubPath = "globalConfig"

	appGlobalConfigFields = "enabled,missingRequiredSettings,globalSettings," +
		"visibilitySettings(permittedGroups(id,name,$type),permittedUsers(id,login,$type))"
	appGlobalConfigFieldsParam = "fields=globalConfig(" + appGlobalConfigFields + ")"

	appGlobalConfigWriteFields      = "enabled,globalSettings"
	appGlobalConfigWriteFieldsParam = "fields=" + appGlobalConfigWriteFields

	appGlobalConfigWriteFmt = "%s/%s/%s/%s?%s"
)

// AppVisibilityGroup is a user group allowed to use an app. YouTrack reports
// the group of all registered users with its own $type, so Type is kept.
type AppVisibilityGroup struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Type string `json:"$type,omitempty"`
}

// AppVisibilityUser is a single user allowed to use an app.
type AppVisibilityUser struct {
	ID    string `json:"id,omitempty"`
	Login string `json:"login,omitempty"`
	Type  string `json:"$type,omitempty"`
}

// AppVisibilitySettings lists who may use an app.
type AppVisibilitySettings struct {
	PermittedGroups []AppVisibilityGroup `json:"permittedGroups"`
	PermittedUsers  []AppVisibilityUser  `json:"permittedUsers"`
	Type            string               `json:"$type,omitempty"`
}

// AppGlobalConfig is the instance-wide configuration of an app: whether it is
// enabled, its global settings, and who may use it.
//
// GlobalSettings is the JSON document YouTrack stores for the settings
// declared in the app's settings.json, as a string. It is empty when the app
// has none.
type AppGlobalConfig struct {
	Enabled                 bool                   `json:"enabled"`
	MissingRequiredSettings bool                   `json:"missingRequiredSettings"`
	GlobalSettings          string                 `json:"globalSettings,omitempty"`
	VisibilitySettings      *AppVisibilitySettings `json:"visibilitySettings,omitempty"`
	Type                    string                 `json:"$type,omitempty"`
}

// AppGlobalConfigUpdate selects what SetAppGlobalConfig changes. A nil field is
// left as it is.
type AppGlobalConfigUpdate struct {
	Enabled        *bool   `json:"enabled,omitempty"`
	GlobalSettings *string `json:"globalSettings,omitempty"`
}

type appGlobalConfigEnvelope struct {
	GlobalConfig *AppGlobalConfig `json:"globalConfig"`
}

type appVisibilityPayload struct {
	GlobalConfig struct {
		VisibilitySettings AppVisibilitySettings `json:"visibilitySettings"`
	} `json:"globalConfig"`
}

// GetAppGlobalConfig reads the global configuration of an app by its entity ID.
func (c *Client) GetAppGlobalConfig(ctx context.Context, appID string) (*AppGlobalConfig, error) {
	endpoint := fmt.Sprintf(resourceByIDWithFieldsFormat, c.HostURL, appsAPIPath, url.PathEscape(appID), appGlobalConfigFieldsParam)

	req, err := http.NewRequestWithContext(ctx, httpMethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get app global config request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get app global config: %w", err)
	}

	var envelope appGlobalConfigEnvelope
	if err = json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("failed to unmarshal app global config response: %w", err)
	}
	if envelope.GlobalConfig == nil {
		return nil, fmt.Errorf("app %s has no global config in the response", appID)
	}

	return envelope.GlobalConfig, nil
}

// SetAppGlobalConfig enables or disables an app and/or replaces its global
// settings. GlobalSettings replaces the whole settings document: a setting left
// out of it is cleared, a secret included. The response carries only Enabled
// and GlobalSettings, with secrets masked.
func (c *Client) SetAppGlobalConfig(ctx context.Context, appID string, update AppGlobalConfigUpdate) (*AppGlobalConfig, error) {
	rb, err := json.Marshal(update)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal app global config payload: %w", err)
	}

	endpoint := fmt.Sprintf(appGlobalConfigWriteFmt, c.HostURL, appsAPIPath, url.PathEscape(appID),
		appGlobalConfigSubPath, appGlobalConfigWriteFieldsParam)

	req, err := http.NewRequestWithContext(ctx, httpMethodPost, endpoint, bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create set app global config request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to set app global config: %w", err)
	}

	var config AppGlobalConfig
	if err = json.Unmarshal(body, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal app global config response: %w", err)
	}

	return &config, nil
}

// SetAppVisibility replaces the groups and users allowed to use an app. An
// empty list for both leaves the app usable by nobody.
func (c *Client) SetAppVisibility(ctx context.Context, appID string, groupIDs, userIDs []string) (*AppVisibilitySettings, error) {
	var payload appVisibilityPayload
	visibility := &payload.GlobalConfig.VisibilitySettings
	visibility.PermittedGroups = make([]AppVisibilityGroup, 0, len(groupIDs))
	for _, id := range groupIDs {
		visibility.PermittedGroups = append(visibility.PermittedGroups, AppVisibilityGroup{ID: id})
	}
	visibility.PermittedUsers = make([]AppVisibilityUser, 0, len(userIDs))
	for _, id := range userIDs {
		visibility.PermittedUsers = append(visibility.PermittedUsers, AppVisibilityUser{ID: id})
	}

	rb, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal app visibility payload: %w", err)
	}

	endpoint := fmt.Sprintf(resourceByIDWithFieldsFormat, c.HostURL, appsAPIPath, url.PathEscape(appID), appGlobalConfigFieldsParam)

	req, err := http.NewRequestWithContext(ctx, httpMethodPost, endpoint, bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create set app visibility request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to set app visibility: %w", err)
	}

	var envelope appGlobalConfigEnvelope
	if err = json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("failed to unmarshal app visibility response: %w", err)
	}
	if envelope.GlobalConfig == nil || envelope.GlobalConfig.VisibilitySettings == nil {
		return nil, fmt.Errorf("app %s has no visibility settings in the response", appID)
	}

	return envelope.GlobalConfig.VisibilitySettings, nil
}
