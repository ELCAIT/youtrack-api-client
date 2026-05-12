package youtrack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	localePath = "%s/api/admin/globalSettings/localeSettings?fields=locale(id,locale,language,community,name)"
)

// GetLocaleSettings - Returns locale settings.
func (c *Client) GetLocaleSettings(ctx context.Context) (LocaleSettings, error) {
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, fmt.Sprintf(localePath, c.HostURL), nil)
	if err != nil {
		return LocaleSettings{}, fmt.Errorf("failed to create get locale settings request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return LocaleSettings{}, fmt.Errorf("failed to get locale settings: %w", err)
	}

	var response LocaleSettings
	err = json.Unmarshal(body, &response)
	if err != nil {
		return LocaleSettings{}, fmt.Errorf("failed to unmarshal locale settings response: %w", err)
	}

	return response, nil
}

// UpdateLocaleSettings - Updates existing locale settings.
func (c *Client) UpdateLocaleSettings(ctx context.Context, localeSettings LocaleSettings) (LocaleSettings, error) {
	requestBody := map[string]interface{}{
		"locale": localeSettings.Locale,
	}

	rb, err := json.Marshal(requestBody)
	if err != nil {
		return LocaleSettings{}, fmt.Errorf("failed to marshal locale settings request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, httpMethodPost, fmt.Sprintf(localePath, c.HostURL), bytes.NewReader(rb))
	if err != nil {
		return LocaleSettings{}, fmt.Errorf("failed to create update locale settings request: %w", err)
	}

	_, err = c.doRequest(req)
	if err != nil {
		return LocaleSettings{}, fmt.Errorf("failed to update locale settings: %w", err)
	}

	// Wait for the API to process the change (async processing)
	waitForAsyncProcessing()

	// Read back the updated settings to get the actual current state
	return c.GetLocaleSettings(ctx)
}
