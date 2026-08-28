package youtrack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	restPath = "%s/api/admin/globalSettings/restSettings?fields=id,allowedOrigins,allowAllOrigins"
)

// GetRestSettings - Returns REST API settings.
func (c *Client) GetRestSettings(ctx context.Context) (RestSettings, error) {
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, fmt.Sprintf(restPath, c.HostURL), nil)
	if err != nil {
		return RestSettings{}, fmt.Errorf("failed to create get REST settings request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return RestSettings{}, fmt.Errorf("failed to get REST settings: %w", err)
	}

	var response RestSettings
	err = json.Unmarshal(body, &response)
	if err != nil {
		return RestSettings{}, fmt.Errorf("failed to unmarshal REST settings response: %w", err)
	}

	return response, nil
}

// UpdateRestSettings - Updates existing REST API settings.
func (c *Client) UpdateRestSettings(ctx context.Context, restSettings RestSettings) (RestSettings, error) {

	rb, err := json.Marshal(restSettings)
	if err != nil {
		return RestSettings{}, fmt.Errorf("failed to marshal REST settings request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, httpMethodPost, fmt.Sprintf(restPath, c.HostURL), bytes.NewReader(rb))
	if err != nil {
		return RestSettings{}, fmt.Errorf("failed to create update REST settings request: %w", err)
	}

	_, err = c.doRequest(req)
	if err != nil {
		return RestSettings{}, fmt.Errorf("failed to update REST settings: %w", err)
	}

	// The write is applied asynchronously, so read back until the reported
	// origins match the ones that were just set.
	return readBackDeepEqual(ctx, c.GetRestSettings,
		func(s RestSettings) restSettingsState {
			return restSettingsState{AllowAllOrigins: s.AllowAllOrigins, AllowedOrigins: s.AllowedOrigins}
		},
		restSettingsState{AllowAllOrigins: restSettings.AllowAllOrigins, AllowedOrigins: restSettings.AllowedOrigins})
}
