package youtrack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

const (
	ownedBundlesAPIPath    = "api/admin/customFieldSettings/bundles/ownedField"
	ownedBundleByIDPath    = "%s/%s/%s?%s"
	ownedBundleFieldsPath  = pathWithFieldsFormat
	ownedBundlePagePath    = "%s/%s?%s&$top=%d&$skip=%d"
	ownedBundleValueFields = "id,name,description,archived,ordinal,owner(id,login,name,$type),$type"
	ownedBundleFieldsParam = "fields=id,name,isUpdateable,values(" + ownedBundleValueFields + "),$type"
	ownedBundleValuesPath  = "values"
	ownedBundlePageSize    = 100
	errMarshalOwnedBundle  = "failed to marshal owned bundle: %w"
	errMarshalOwnedValue   = "failed to marshal owned bundle value: %w"
)

var errOwnedBundleNotFound = fmt.Errorf("owned bundle %w", ErrNotFound)

// OwnedBundleElement represents a single value inside an owned field bundle.
// Unlike enum and state values it has no localized name: it extends
// BundleElement rather than LocalizableBundleElement.
type OwnedBundleElement struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Archived    bool   `json:"archived,omitempty"`
	Ordinal     int    `json:"ordinal,omitempty"`
	// Owner is the user associated with the value. YouTrack allows it to be null.
	Owner *UserRef `json:"owner,omitempty"`
	Type  string   `json:"$type,omitempty"`
}

// OwnedBundleValueUpdate is the request body for UpdateOwnedBundleValue. It
// replaces every field it carries: a nil Description or Owner is sent as null
// and clears it, and Archived is always sent. Ordinal is only sent when set.
//
// A bundle-level update cannot do this: YouTrack ignores changes to values that
// already exist when they arrive in UpdateOwnedBundle's values list, so edits to
// an existing value have to go through this per-value endpoint.
type OwnedBundleValueUpdate struct {
	Name        string   `json:"name"`
	Description *string  `json:"description"`
	Archived    bool     `json:"archived"`
	Ordinal     *int     `json:"ordinal,omitempty"`
	Owner       *UserRef `json:"owner"`
}

// OwnedBundle represents a YouTrack owned field bundle.
type OwnedBundle struct {
	ID           string               `json:"id,omitempty"`
	Name         string               `json:"name,omitempty"`
	IsUpdateable bool                 `json:"isUpdateable,omitempty"`
	Values       []OwnedBundleElement `json:"values,omitempty"`
	Type         string               `json:"$type,omitempty"`
}

// GetOwnedBundleByID returns a specific owned field bundle.
func (c *Client) GetOwnedBundleByID(ctx context.Context, id string) (*OwnedBundle, error) {
	return fetchAndDecodeByID[OwnedBundle](
		ctx,
		c,
		idFetchConfig{
			PathFormat: ownedBundleByIDPath,
			HostURL:    c.HostURL,
			APIPath:    ownedBundlesAPIPath,
			Fields:     ownedBundleFieldsParam,
			ErrCreate:  "failed to create get owned bundle request: %w",
			ErrFetch:   "failed to get owned bundle: %w",
			ErrDecode:  "failed to unmarshal owned bundle response: %w",
		},
		id,
	)
}

// GetOwnedBundleByName returns an owned field bundle by name.
func (c *Client) GetOwnedBundleByName(ctx context.Context, name string) (*OwnedBundle, error) {
	bundle, err := lookupByNamePaginated(ctx, ownedBundlePageSize, name, c.getOwnedBundlePage, ownedBundleName)
	if err != nil {
		return nil, err
	}
	if bundle != nil {
		return bundle, nil
	}

	return nil, fmt.Errorf("%w: name '%s'", errOwnedBundleNotFound, name)
}

func (c *Client) getOwnedBundlePage(ctx context.Context, skip int) ([]OwnedBundle, error) {
	return fetchAndDecodePage(
		ctx,
		c,
		pageFetchConfig{
			PathFormat: ownedBundlePagePath,
			HostURL:    c.HostURL,
			APIPath:    ownedBundlesAPIPath,
			Fields:     ownedBundleFieldsParam,
			PageSize:   ownedBundlePageSize,
			ErrCreate:  "failed to create get owned bundles request: %w",
			ErrFetch:   "failed to get owned bundles: %w",
		},
		skip,
		decodeOwnedBundles,
	)
}

func ownedBundleName(bundle OwnedBundle) string {
	return bundle.Name
}

func decodeOwnedBundles(body []byte) ([]OwnedBundle, error) {
	return decodeBundleList[OwnedBundle](body, "failed to unmarshal owned bundles response: %w")
}

// IsOwnedBundleNotFoundError checks whether an error indicates that an owned bundle could not be found by name.
func IsOwnedBundleNotFoundError(err error) bool {
	return errors.Is(err, errOwnedBundleNotFound)
}

// CreateOwnedBundle creates a new owned field bundle.
func (c *Client) CreateOwnedBundle(ctx context.Context, bundle OwnedBundle) (*OwnedBundle, error) {
	return createAndDecode(ctx, c, bundle, createConfig{
		PathFormat: ownedBundleFieldsPath,
		HostURL:    c.HostURL,
		APIPath:    ownedBundlesAPIPath,
		Fields:     ownedBundleFieldsParam,
		ErrMarshal: errMarshalOwnedBundle,
		ErrCreate:  "failed to create create owned bundle request: %w",
		ErrFetch:   "failed to create owned bundle: %w",
		ErrDecode:  "failed to unmarshal created owned bundle: %w",
	})
}

// UpdateOwnedBundle updates a specific owned field bundle by ID.
func (c *Client) UpdateOwnedBundle(ctx context.Context, id string, bundle OwnedBundle) (*OwnedBundle, error) {
	return updateAndDecode(ctx, c, id, bundle, updateConfig{
		PathFormat: ownedBundleByIDPath,
		HostURL:    c.HostURL,
		APIPath:    ownedBundlesAPIPath,
		Fields:     ownedBundleFieldsParam,
		ErrMarshal: errMarshalOwnedBundle,
		ErrCreate:  "failed to create update owned bundle request: %w",
		ErrFetch:   "failed to update owned bundle: %w",
		ErrDecode:  "failed to unmarshal updated owned bundle: %w",
	})
}

// DeleteOwnedBundle deletes an owned field bundle by ID.
func (c *Client) DeleteOwnedBundle(ctx context.Context, id string) error {
	return deleteByID(ctx, c, id, deleteConfig{
		HostURL:   c.HostURL,
		APIPath:   ownedBundlesAPIPath,
		ErrCreate: "failed to create delete owned bundle request: %w",
		ErrFetch:  "failed to delete owned bundle: %w",
	})
}

// AddOwnedBundleValue adds a value to an owned field bundle.
func (c *Client) AddOwnedBundleValue(ctx context.Context, bundleID string, value OwnedBundleElement) (*OwnedBundleElement, error) {
	endpoint := c.buildURL(ownedBundlesAPIPath, []string{bundleID, ownedBundleValuesPath}, fieldsQuery(ownedBundleValueFields))
	return c.postOwnedBundleValue(ctx, endpoint, value, "add")
}

// UpdateOwnedBundleValue replaces the fields of one value in an owned field bundle.
func (c *Client) UpdateOwnedBundleValue(ctx context.Context, bundleID, valueID string, value OwnedBundleValueUpdate) (*OwnedBundleElement, error) {
	endpoint := c.buildURL(ownedBundlesAPIPath, []string{bundleID, ownedBundleValuesPath, valueID}, fieldsQuery(ownedBundleValueFields))
	return c.postOwnedBundleValue(ctx, endpoint, value, "update")
}

// DeleteOwnedBundleValue removes one value from an owned field bundle. A value
// that is already gone is treated as deleted.
func (c *Client) DeleteOwnedBundleValue(ctx context.Context, bundleID, valueID string) error {
	endpoint := c.buildURL(ownedBundlesAPIPath, []string{bundleID, ownedBundleValuesPath, valueID}, nil)
	req, err := http.NewRequestWithContext(ctx, httpMethodDelete, endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete owned bundle value request: %w", err)
	}

	if _, err := c.doRequest(req); err != nil && !IsNotFoundError(err) {
		return fmt.Errorf("failed to delete owned bundle value: %w", err)
	}

	return nil
}

func (c *Client) postOwnedBundleValue(ctx context.Context, endpoint string, payload any, action string) (*OwnedBundleElement, error) {
	rb, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf(errMarshalOwnedValue, err)
	}

	req, err := http.NewRequestWithContext(ctx, httpMethodPost, endpoint, bytes.NewReader(rb))
	if err != nil {
		return nil, fmt.Errorf("failed to create %s owned bundle value request: %w", action, err)
	}

	body, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to %s owned bundle value: %w", action, err)
	}

	var value OwnedBundleElement
	if err := json.Unmarshal(body, &value); err != nil {
		return nil, fmt.Errorf("failed to unmarshal %s owned bundle value response: %w", action, err)
	}

	return &value, nil
}
