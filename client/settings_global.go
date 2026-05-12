package youtrack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	globalPath = "%s/api/admin/globalSettings?fields=id,license(id,username,license,error)"
)

// GetGlobalSettings - Returns global settings.
func (c *Client) GetGlobalSettings(ctx context.Context) (GlobalSettings, error) {
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, fmt.Sprintf(globalPath, c.HostURL), nil)
	if err != nil {
		return GlobalSettings{}, fmt.Errorf("failed to create get global settings request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return GlobalSettings{}, fmt.Errorf("failed to get global settings: %w", err)
	}

	var response GlobalSettings
	err = json.Unmarshal(body, &response)
	if err != nil {
		return GlobalSettings{}, fmt.Errorf("failed to unmarshal global settings response: %w", err)
	}

	return response, nil
}

// UpdateGlobalSettings - Updates existing global settings.
// Note: The API expects the license as an object with $type field.
func (c *Client) UpdateGlobalSettings(ctx context.Context, globalSettings GlobalSettings) (GlobalSettings, error) {
	// Build request body with updateable fields
	requestBody := make(map[string]interface{})

	if globalSettings.License != nil {
		// Send license as an object with the required $type field
		requestBody["license"] = map[string]interface{}{
			"$type":   "jetbrains.charisma.persistent.globalSettings.License",
			"license": globalSettings.License.License,
		}
	} else {
		requestBody["license"] = nil
	}

	// Future fields can be added here as needed
	// Example: if globalSettings.SomeOtherField != "" {
	//     requestBody["someOtherField"] = globalSettings.SomeOtherField
	// }

	rb, err := json.Marshal(requestBody)
	if err != nil {
		return GlobalSettings{}, fmt.Errorf("failed to marshal global settings request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, httpMethodPost, fmt.Sprintf(globalPath, c.HostURL), bytes.NewReader(rb))
	if err != nil {
		return GlobalSettings{}, fmt.Errorf("failed to create update global settings request: %w", err)
	}

	_, err = c.doRequest(req)
	if err != nil {
		return GlobalSettings{}, fmt.Errorf("failed to update global settings: %w", err)
	}

	// Wait for the API to process the change (async processing)
	waitForAsyncProcessing()

	// Read back the updated settings to get the actual current state
	return c.GetGlobalSettings(ctx)
}
