package youtrack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

const (
	azureAuthModuleFieldsParam = "fields=$type,id,name,disabled,default,clientId,redirectUri,iconUrl,extensionGrantType," +
		"serverUrl,connectionTimeout,readTimeout,backgroundSyncEnabled,syncInterval," +
		"allowedCreateNewUsers,tenant,requestGroupPermission,requestIdToken"
)

// CreateAzureAuthModule creates a new Microsoft Entra ID auth module in Hub.
func (c *Client) CreateAzureAuthModule(ctx context.Context, module AzureAuthModule) (*AzureAuthModule, error) {
	module.Type = azureAuthModuleType

	rb, err := json.Marshal(module) //nolint:gosec // ClientSecret is intentionally sent to the API
	if err != nil {
		return nil, fmt.Errorf("failed to marshal azure auth module: %w", err)
	}

	endpoint := fmt.Sprintf(authModulesBasePath+"?%s", c.HostURL, apiBasePath, azureAuthModuleFieldsParam)
	req, err := http.NewRequestWithContext(ctx, httpMethodPost, endpoint, bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create azure auth module request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create azure auth module: %w", err)
	}

	var created AzureAuthModule
	if err = json.Unmarshal(body, &created); err != nil {
		return nil, fmt.Errorf("failed to unmarshal created azure auth module: %w", err)
	}

	return &created, nil
}

// GetAzureAuthModuleByID retrieves a Microsoft Entra ID auth module by its ID.
func (c *Client) GetAzureAuthModuleByID(ctx context.Context, moduleID string) (*AzureAuthModule, error) {
	endpoint := fmt.Sprintf(authModuleByIDPath+"?%s", c.HostURL, apiBasePath, url.PathEscape(moduleID), azureAuthModuleFieldsParam)
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get azure auth module request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get azure auth module: %w", err)
	}

	var module AzureAuthModule
	if err = json.Unmarshal(body, &module); err != nil {
		return nil, fmt.Errorf("failed to unmarshal azure auth module: %w", err)
	}

	return &module, nil
}

// UpdateAzureAuthModule updates an existing Microsoft Entra ID auth module. Hub uses POST for updates.
func (c *Client) UpdateAzureAuthModule(ctx context.Context, moduleID string, module AzureAuthModule) (*AzureAuthModule, error) {
	module.Type = azureAuthModuleType

	rb, err := json.Marshal(module) //nolint:gosec // ClientSecret is intentionally sent to the API
	if err != nil {
		return nil, fmt.Errorf("failed to marshal azure auth module update: %w", err)
	}

	endpoint := fmt.Sprintf(authModuleByIDPath+"?%s", c.HostURL, apiBasePath, url.PathEscape(moduleID), azureAuthModuleFieldsParam)
	req, err := http.NewRequestWithContext(ctx, httpMethodPost, endpoint, bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create update azure auth module request: %w", err)
	}

	_, err = c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to update azure auth module: %w", err)
	}

	// Hub returns an empty body on a successful update and applies it
	// asynchronously; wait for it to settle before fetching the updated
	// state, matching UpdateOAuth2AuthModule.
	waitForAsyncProcessing()

	return c.GetAzureAuthModuleByID(ctx, moduleID)
}

// DeleteAzureAuthModule deletes a Microsoft Entra ID auth module by its ID.
func (c *Client) DeleteAzureAuthModule(ctx context.Context, moduleID string) error {
	endpoint := fmt.Sprintf(authModuleByIDPath, c.HostURL, apiBasePath, url.PathEscape(moduleID))
	req, err := http.NewRequestWithContext(ctx, httpMethodDelete, endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete azure auth module request: %w", err)
	}

	_, err = c.doRequest(req)
	if err != nil {
		return fmt.Errorf("failed to delete azure auth module: %w", err)
	}

	return nil
}
