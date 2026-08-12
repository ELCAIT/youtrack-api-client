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
	servicesBasePath = "%s/%s/services"
	serviceByIDPath  = "%s/%s/services/%s"
	serviceFields    = "id,type,name,key,homeUrl,applicationName,description,vendor,version,trusted,consentRequired,secret,redirectUris,baseUrls," +
		"clientCredentialsFlowEnabled,authCodeFlowEnabled,pkceRequired,implicitFlowEnabled,resourceOwnerFlowEnabled"
	serviceFieldsQueryParam = "fields=" + serviceFields
)

// CreateService registers a new Hub service (an external application entry).
func (c *Client) CreateService(ctx context.Context, service Service) (*Service, error) {
	rb, err := json.Marshal(service) //nolint:gosec // Secret is intentionally sent to the API
	if err != nil {
		return nil, fmt.Errorf("failed to marshal service: %w", err)
	}

	endpoint := fmt.Sprintf(servicesBasePath+"?%s", c.HostURL, apiBasePath, serviceFieldsQueryParam)
	req, err := http.NewRequestWithContext(ctx, httpMethodPost, endpoint, bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create service request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create service: %w", err)
	}

	var created Service
	if err = json.Unmarshal(body, &created); err != nil {
		return nil, fmt.Errorf("failed to unmarshal created service: %w", err)
	}

	return &created, nil
}

// ListServices returns Hub services and supports optional pagination via top/skip.
// Pass 0 for top and skip to use the default server-side pagination.
func (c *Client) ListServices(ctx context.Context, top, skip int) ([]Service, error) {
	query := withPagination(serviceFields, top, skip)
	endpoint := fmt.Sprintf(servicesBasePath+"?%s", c.HostURL, apiBasePath, query)
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list services request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	var response ServicesResponse
	if err = json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal services response: %w", err)
	}

	return response.Services, nil
}

// GetServiceByID retrieves a Hub service by its ID.
func (c *Client) GetServiceByID(ctx context.Context, serviceID string) (*Service, error) {
	endpoint := fmt.Sprintf(serviceByIDPath+"?%s", c.HostURL, apiBasePath, url.PathEscape(serviceID), serviceFieldsQueryParam)
	req, err := http.NewRequestWithContext(ctx, httpMethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get service request: %w", err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get service: %w", err)
	}

	var service Service
	if err = json.Unmarshal(body, &service); err != nil {
		return nil, fmt.Errorf("failed to unmarshal service: %w", err)
	}

	return &service, nil
}

// UpdateService updates an existing Hub service. Hub uses POST for updates
// and returns an empty body on success, so the updated service is re-fetched.
func (c *Client) UpdateService(ctx context.Context, serviceID string, service Service) (*Service, error) {
	rb, err := json.Marshal(service) //nolint:gosec // Secret is intentionally sent to the API
	if err != nil {
		return nil, fmt.Errorf("failed to marshal service update: %w", err)
	}

	endpoint := fmt.Sprintf(serviceByIDPath+"?%s", c.HostURL, apiBasePath, url.PathEscape(serviceID), serviceFieldsQueryParam)
	req, err := http.NewRequestWithContext(ctx, httpMethodPost, endpoint, bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create update service request: %w", err)
	}

	if _, err = c.doRequest(req); err != nil {
		return nil, fmt.Errorf("failed to update service: %w", err)
	}

	return c.GetServiceByID(ctx, serviceID)
}

// DeleteService deletes a Hub service by ID.
func (c *Client) DeleteService(ctx context.Context, serviceID string) error {
	endpoint := fmt.Sprintf(serviceByIDPath, c.HostURL, apiBasePath, url.PathEscape(serviceID))
	req, err := http.NewRequestWithContext(ctx, httpMethodDelete, endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete service request: %w", err)
	}

	_, err = c.doRequest(req)
	if err != nil && !IsNotFoundError(err) {
		return fmt.Errorf("failed to delete service: %w", err)
	}

	return nil
}
