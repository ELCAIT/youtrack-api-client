package youtrack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	appearancePath = "%s/api/admin/globalSettings/appearanceSettings?fields=id,timeZone(id,presentation,offset),dateFieldFormat(id,presentation,pattern,datePattern),logo(id,url)"
)

// GetAppearanceSettings - Returns appearance settings.
func (c *Client) GetAppearanceSettings(ctx context.Context) (AppearanceSettings, error) {
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, fmt.Sprintf(appearancePath, c.HostURL), nil)
	if err != nil {
		return AppearanceSettings{}, fmt.Errorf("failed to create get appearance settings request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return AppearanceSettings{}, fmt.Errorf("failed to get appearance settings: %w", err)
	}

	var response AppearanceSettings
	err = json.Unmarshal(body, &response)
	if err != nil {
		return AppearanceSettings{}, fmt.Errorf("failed to unmarshal appearance settings response: %w", err)
	}

	return response, nil
}

// UpdateAppearanceSettings - Updates existing appearance settings. Only the
// fields set on appearanceSettings (non-empty ID) are sent, so leaving one
// field zero-valued doesn't clear its previously configured value.
func (c *Client) UpdateAppearanceSettings(ctx context.Context, appearanceSettings AppearanceSettings) (AppearanceSettings, error) {
	requestBody := map[string]interface{}{}
	if appearanceSettings.DateFormat.ID != "" {
		requestBody["dateFieldFormat"] = map[string]interface{}{
			"id": appearanceSettings.DateFormat.ID,
		}
	}
	if appearanceSettings.TimeZone.ID != "" {
		requestBody["timeZone"] = map[string]interface{}{
			"id": appearanceSettings.TimeZone.ID,
		}
	}

	rb, err := json.Marshal(requestBody)
	if err != nil {
		return AppearanceSettings{}, fmt.Errorf("failed to marshal appearance settings request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, httpMethodPost, fmt.Sprintf(appearancePath, c.HostURL), bytes.NewReader(rb))
	if err != nil {
		return AppearanceSettings{}, fmt.Errorf("failed to create update appearance settings request: %w", err)
	}

	_, err = c.doRequest(req)
	if err != nil {
		return AppearanceSettings{}, fmt.Errorf("failed to update appearance settings: %w", err)
	}

	// The write is applied asynchronously, so read back until the reported
	// settings match those that were just set. Only the fields the caller
	// actually sent are compared, matching this method's partial-update
	// semantics: a field left zero-valued was not written and must not be
	// waited on.
	want := appearanceSettingsState{
		DateFormatID: appearanceSettings.DateFormat.ID,
		TimeZoneID:   appearanceSettings.TimeZone.ID,
	}

	return readBackEqual(ctx, c.GetAppearanceSettings,
		func(s AppearanceSettings) appearanceSettingsState {
			return appearanceSettingsState{
				DateFormatID: observedOrSkip(want.DateFormatID, s.DateFormat.ID),
				TimeZoneID:   observedOrSkip(want.TimeZoneID, s.TimeZone.ID),
			}
		}, want)
}
