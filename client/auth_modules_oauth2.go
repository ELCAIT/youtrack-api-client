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
	authModulesBasePath   = "%s/%s/authmodules"
	authModuleByIDPath    = "%s/%s/authmodules/%s"
	authModuleFieldsParam = "fields=$type,id,name,disabled,default,clientId,redirectUri,iconUrl,extensionGrantType," +
		"serverUrl,connectionTimeout,readTimeout,backgroundSyncEnabled,syncInterval," +
		"allowedCreateNewUsers,scope,tokenUrl,formClientAuth,userInfoUrl,idpLogoutUrl," +
		"userIdPath,userEmailUrl,userAvatarUrl,userEmailPath,userEmailVerifiedPath," +
		"userNamePath,fullNamePath,userPictureIdPath,userPictureUrlPattern," +
		"emailVerifiedByDefault,userGroupsPath"
)

// CreateOAuth2AuthModule creates a new OAuth 2.0 auth module in Hub.
func (c *Client) CreateOAuth2AuthModule(ctx context.Context, module OAuth2AuthModule) (*OAuth2AuthModule, error) {
	module.Type = oauth2AuthModuleType

	rb, err := json.Marshal(module) //nolint:gosec // ClientSecret is intentionally sent to the API
	if err != nil {
		return nil, fmt.Errorf("failed to marshal oauth2 auth module: %w", err)
	}

	endpoint := fmt.Sprintf(authModulesBasePath+"?%s", c.HostURL, apiBasePath, authModuleFieldsParam)
	req, err := http.NewRequestWithContext(ctx, httpMethodPost, endpoint, bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create oauth2 auth module request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create oauth2 auth module: %w", err)
	}

	var created OAuth2AuthModule
	if err = json.Unmarshal(body, &created); err != nil {
		return nil, fmt.Errorf("failed to unmarshal created oauth2 auth module: %w", err)
	}

	return &created, nil
}

// GetOAuth2AuthModuleByID retrieves an OAuth 2.0 auth module by its ID.
func (c *Client) GetOAuth2AuthModuleByID(ctx context.Context, moduleID string) (*OAuth2AuthModule, error) {
	endpoint := fmt.Sprintf(authModuleByIDPath+"?%s", c.HostURL, apiBasePath, url.PathEscape(moduleID), authModuleFieldsParam)
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get oauth2 auth module request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get oauth2 auth module: %w", err)
	}

	var module OAuth2AuthModule
	if err = json.Unmarshal(body, &module); err != nil {
		return nil, fmt.Errorf("failed to unmarshal oauth2 auth module: %w", err)
	}

	return &module, nil
}

// UpdateOAuth2AuthModule updates an existing OAuth 2.0 auth module. Hub uses POST for updates.
func (c *Client) UpdateOAuth2AuthModule(ctx context.Context, moduleID string, module OAuth2AuthModule) (*OAuth2AuthModule, error) {
	module.Type = oauth2AuthModuleType

	rb, err := json.Marshal(module) //nolint:gosec // ClientSecret is intentionally sent to the API
	if err != nil {
		return nil, fmt.Errorf("failed to marshal oauth2 auth module update: %w", err)
	}

	endpoint := fmt.Sprintf(authModuleByIDPath+"?%s", c.HostURL, apiBasePath, url.PathEscape(moduleID), authModuleFieldsParam)
	req, err := http.NewRequestWithContext(ctx, httpMethodPost, endpoint, bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create update oauth2 auth module request: %w", err)
	}

	_, err = c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to update oauth2 auth module: %w", err)
	}

	// Hub returns an empty body on a successful update; fetch the updated state.
	return c.GetOAuth2AuthModuleByID(ctx, moduleID)
}

// DeleteOAuth2AuthModule deletes an OAuth 2.0 auth module by its ID.
func (c *Client) DeleteOAuth2AuthModule(ctx context.Context, moduleID string) error {
	endpoint := fmt.Sprintf(authModuleByIDPath, c.HostURL, apiBasePath, url.PathEscape(moduleID))
	req, err := http.NewRequestWithContext(ctx, httpMethodDelete, endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete oauth2 auth module request: %w", err)
	}

	_, err = c.doRequest(req)
	if err != nil {
		return fmt.Errorf("failed to delete oauth2 auth module: %w", err)
	}

	return nil
}
