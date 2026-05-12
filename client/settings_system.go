package youtrack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	settingsPath = "%s/api/admin/globalSettings/systemSettings?fields=administratorEmail,maxExportItems,maxUploadFileSize,allowStatisticsCollection,isApplicationReadOnly,baseUrl"
)

// Get System Settings - Return System Settings.
func (c *Client) GetSystemSettings(ctx context.Context) (SystemSettings, error) {
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, fmt.Sprintf(settingsPath, c.HostURL), nil)
	if err != nil {
		return SystemSettings{}, fmt.Errorf("failed to create get system settings request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return SystemSettings{}, fmt.Errorf("failed to get system settings: %w", err)
	}

	var response SystemSettings
	err = json.Unmarshal(body, &response)
	if err != nil {
		return SystemSettings{}, fmt.Errorf("failed to unmarshal system settings response: %w", err)
	}

	return response, nil
}

// Update System Settings - Updates existing system settings.
func (c *Client) UpdateSystemSettings(ctx context.Context, systemSettings SystemSettings) (SystemSettings, error) {
	rb, err := json.Marshal(systemSettings)
	if err != nil {
		return SystemSettings{}, fmt.Errorf("failed to marshal system settings request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, httpMethodPost, fmt.Sprintf(settingsPath, c.HostURL), bytes.NewReader(rb))
	if err != nil {
		return SystemSettings{}, fmt.Errorf("failed to create update system settings request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return SystemSettings{}, fmt.Errorf("failed to update system settings: %w", err)
	}

	var response SystemSettings
	err = json.Unmarshal(body, &response)
	if err != nil {
		return SystemSettings{}, fmt.Errorf("failed to unmarshal system settings response: %w", err)
	}

	return response, nil
}
