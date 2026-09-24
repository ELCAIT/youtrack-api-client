package youtrack

import (
	"context"
	"errors"
	"fmt"
)

const (
	ownedBundlesAPIPath    = "api/admin/customFieldSettings/bundles/ownedField"
	ownedBundleByIDPath    = "%s/%s/%s?%s"
	ownedBundleFieldsPath  = pathWithFieldsFormat
	ownedBundlePagePath    = "%s/%s?%s&$top=%d&$skip=%d"
	ownedBundleValueFields = "id,name,description,archived,ordinal,owner(id,login,name,$type),$type"
	ownedBundleFieldsParam = "fields=id,name,isUpdateable,values(" + ownedBundleValueFields + "),$type"
	ownedBundlePageSize    = 100
	errMarshalOwnedBundle  = "failed to marshal owned bundle: %w"
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

// OwnedBundleValueUpdate is the request body for ReplaceOwnedBundleValue. It
// replaces every field it carries: a nil Description or Owner is sent as null
// and clears it, and Archived is always sent. Ordinal is only sent when set.
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
// While YouTrack still reports the bundle as in use, shortly after the last
// field using it was removed, the delete is retried.
func (c *Client) DeleteOwnedBundle(ctx context.Context, id string) error {
	return deleteBundleByID(ctx, c, id, deleteConfig{
		HostURL:   c.HostURL,
		APIPath:   ownedBundlesAPIPath,
		ErrCreate: "failed to create delete owned bundle request: %w",
		ErrFetch:  "failed to delete owned bundle: %w",
	})
}

var ownedBundleValues = bundleValueEndpoint{APIPath: ownedBundlesAPIPath, Fields: ownedBundleValueFields, Kind: "owned"}

// AddOwnedBundleValue adds a value to an owned field bundle.
func (c *Client) AddOwnedBundleValue(ctx context.Context, bundleID string, value OwnedBundleElement) (*OwnedBundleElement, error) {
	return addBundleValue[OwnedBundleElement](ctx, c, ownedBundleValues, bundleID, value)
}

// ReplaceOwnedBundleValue replaces the fields of one value in an owned field bundle.
func (c *Client) ReplaceOwnedBundleValue(ctx context.Context, bundleID, valueID string, value OwnedBundleValueUpdate) (*OwnedBundleElement, error) {
	return replaceBundleValue[OwnedBundleElement](ctx, c, ownedBundleValues, bundleID, valueID, value)
}

// DeleteOwnedBundleValue removes one value from an owned field bundle, waiting
// until YouTrack no longer lists it. A value that is already gone is treated as
// deleted.
func (c *Client) DeleteOwnedBundleValue(ctx context.Context, bundleID, valueID string) error {
	return deleteBundleValue(ctx, c, ownedBundleValues, bundleID, valueID, func(ctx context.Context) (bool, error) {
		bundle, err := c.GetOwnedBundleByID(ctx, bundleID)
		if err != nil {
			return false, err
		}
		return bundleListsValue(bundle.Values, valueID, func(v OwnedBundleElement) string { return v.ID }), nil
	})
}
