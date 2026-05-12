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

// UpdateAppearanceSettings - Updates existing appearance settings.
func (c *Client) UpdateAppearanceSettings(ctx context.Context, appearanceSettings AppearanceSettings) (AppearanceSettings, error) {
	requestBody := map[string]interface{}{
		"dateFieldFormat": map[string]interface{}{
			"id": appearanceSettings.DateFormat.ID,
		},
		"timeZone": map[string]interface{}{
			"id": appearanceSettings.TimeZone.ID,
		},
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

	// Wait for the API to process the change (async processing)
	waitForAsyncProcessing()

	// Read back the updated settings to get the actual current state
	result, err := c.GetAppearanceSettings(ctx)
	if err != nil {
		return AppearanceSettings{}, err
	}

	return result, nil
}
