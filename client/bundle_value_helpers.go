package youtrack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	bundleValuesPath = "values"

	actionAdd     = "add"
	actionReplace = "replace"
)

// bundleValueEndpoint identifies the values of one kind of bundle: the bundle
// collection's API path, the fields to request for a single value, and the
// kind as it reads in error messages.
type bundleValueEndpoint struct {
	APIPath string
	Fields  string
	Kind    string
}

// addBundleValue adds a value to a bundle through its values collection.
func addBundleValue[T any](ctx context.Context, c *Client, ep bundleValueEndpoint, bundleID string, payload any) (*T, error) {
	endpoint := c.buildURL(ep.APIPath, []string{bundleID, bundleValuesPath}, fieldsQuery(ep.Fields))
	return postBundleValue[T](ctx, c, ep, endpoint, payload, actionAdd)
}

// replaceBundleValue writes payload over one existing value of a bundle.
//
// Edits to an existing value have to go through this per-value endpoint:
// YouTrack ignores changes to existing values when they arrive in a
// bundle-level update, and recreates, under a new ID, any value sent there
// without its ID, which clears it on every issue that used it.
func replaceBundleValue[T any](ctx context.Context, c *Client, ep bundleValueEndpoint, bundleID, valueID string, payload any) (*T, error) {
	endpoint := c.buildURL(ep.APIPath, []string{bundleID, bundleValuesPath, valueID}, fieldsQuery(ep.Fields))
	return postBundleValue[T](ctx, c, ep, endpoint, payload, actionReplace)
}

func postBundleValue[T any](ctx context.Context, c *Client, ep bundleValueEndpoint, endpoint string, payload any, action string) (*T, error) {
	rb, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %s bundle value: %w", ep.Kind, err)
	}

	req, err := http.NewRequestWithContext(ctx, httpMethodPost, endpoint, bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create %s %s bundle value request: %w", action, ep.Kind, err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to %s %s bundle value: %w", action, ep.Kind, err)
	}

	var value T
	if err := json.Unmarshal(body, &value); err != nil {
		return nil, fmt.Errorf("failed to unmarshal %s %s bundle value response: %w", action, ep.Kind, err)
	}

	return &value, nil
}

// deleteBundleValue removes one value from a bundle. A value that is already
// gone is treated as deleted.
//
// YouTrack acknowledges the delete before applying it: a read issued straight
// after still lists the value, and deleting the bundle in that window fails
// with "because it is referenced". stillListed re-reads the bundle and reports
// whether the value is still in it; the delete returns once it no longer is.
func deleteBundleValue(
	ctx context.Context,
	c *Client,
	ep bundleValueEndpoint,
	bundleID, valueID string,
	stillListed func(ctx context.Context) (bool, error),
) error {
	endpoint := c.buildURL(ep.APIPath, []string{bundleID, bundleValuesPath, valueID}, nil)
	req, err := http.NewRequestWithContext(ctx, httpMethodDelete, endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete %s bundle value request: %w", ep.Kind, err)
	}

	if _, err := c.doRequest(req); err != nil {
		if IsNotFoundError(err) {
			return nil
		}
		return fmt.Errorf("failed to delete %s bundle value: %w", ep.Kind, err)
	}

	_, err = readBackAfterWrite(ctx, stillListed, func(listed bool) bool { return !listed })
	if err != nil && !IsNotFoundError(err) {
		return fmt.Errorf("failed to confirm %s bundle value deletion: %w", ep.Kind, err)
	}

	return nil
}

// bundleListsValue reports whether values contains one with the given ID.
func bundleListsValue[T any](values []T, valueID string, idOf func(T) string) bool {
	for _, value := range values {
		if idOf(value) == valueID {
			return true
		}
	}
	return false
}
