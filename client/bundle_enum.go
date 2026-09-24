package youtrack

import (
	"context"
	"errors"
	"fmt"
)

const (
	enumBundlesAPIPath    = "api/admin/customFieldSettings/bundles/enum"
	enumBundleByIDPath    = "%s/%s/%s?%s"
	enumBundleFieldsPath  = pathWithFieldsFormat
	enumBundlePagePath    = "%s/%s?%s&$top=%d&$skip=%d"
	enumBundleValueFields = "id,name,localizedName,description,archived,ordinal,$type"
	enumBundleFieldsParam = "fields=id,name,isUpdateable,values(" + enumBundleValueFields + "),$type"
	enumBundlePageSize    = 100
	errMarshalEnumBundle  = "failed to marshal enum bundle: %w"
)

var errEnumBundleNotFound = fmt.Errorf("enum bundle %w", ErrNotFound)

// EnumBundleElement represents a single value inside an enum bundle.
type EnumBundleElement struct {
	ID            string `json:"id,omitempty"`
	Name          string `json:"name,omitempty"`
	LocalizedName string `json:"localizedName,omitempty"`
	Description   string `json:"description,omitempty"`
	Archived      bool   `json:"archived,omitempty"`
	Ordinal       int    `json:"ordinal,omitempty"`
	Type          string `json:"$type,omitempty"`
}

// EnumBundleValueUpdate is the request body for ReplaceEnumBundleValue. It
// replaces every field it carries: a nil LocalizedName or Description is sent
// as null and clears it, and Archived is always sent. Ordinal is only
// sent when set.
type EnumBundleValueUpdate struct {
	Name          string  `json:"name"`
	LocalizedName *string `json:"localizedName"`
	Description   *string `json:"description"`
	Archived      bool    `json:"archived"`
	Ordinal       *int    `json:"ordinal,omitempty"`
}

// EnumBundle represents a YouTrack enum bundle.
type EnumBundle struct {
	ID           string              `json:"id,omitempty"`
	Name         string              `json:"name,omitempty"`
	IsUpdateable bool                `json:"isUpdateable,omitempty"`
	Values       []EnumBundleElement `json:"values,omitempty"`
	Type         string              `json:"$type,omitempty"`
}

// GetEnumBundleByID returns a specific enum bundle.
func (c *Client) GetEnumBundleByID(ctx context.Context, id string) (*EnumBundle, error) {
	return fetchAndDecodeByID[EnumBundle](
		ctx,
		c,
		idFetchConfig{
			PathFormat: enumBundleByIDPath,
			HostURL:    c.HostURL,
			APIPath:    enumBundlesAPIPath,
			Fields:     enumBundleFieldsParam,
			ErrCreate:  "failed to create get enum bundle request: %w",
			ErrFetch:   "failed to get enum bundle: %w",
			ErrDecode:  "failed to unmarshal enum bundle response: %w",
		},
		id,
	)
}

// GetEnumBundleByName returns an enum bundle by name.
func (c *Client) GetEnumBundleByName(ctx context.Context, name string) (*EnumBundle, error) {
	bundle, err := lookupByNamePaginated(ctx, enumBundlePageSize, name, c.getEnumBundlePage, enumBundleName)
	if err != nil {
		return nil, err
	}
	if bundle != nil {
		return bundle, nil
	}

	return nil, fmt.Errorf("%w: name '%s'", errEnumBundleNotFound, name)
}

func (c *Client) getEnumBundlePage(ctx context.Context, skip int) ([]EnumBundle, error) {
	return fetchAndDecodePage(
		ctx,
		c,
		pageFetchConfig{
			PathFormat: enumBundlePagePath,
			HostURL:    c.HostURL,
			APIPath:    enumBundlesAPIPath,
			Fields:     enumBundleFieldsParam,
			PageSize:   enumBundlePageSize,
			ErrCreate:  "failed to create get enum bundles request: %w",
			ErrFetch:   "failed to get enum bundles: %w",
		},
		skip,
		decodeEnumBundles,
	)
}

func enumBundleName(bundle EnumBundle) string {
	return bundle.Name
}

func decodeEnumBundles(body []byte) ([]EnumBundle, error) {
	return decodeBundleList[EnumBundle](body, "failed to unmarshal enum bundles response: %w")
}

// IsEnumBundleNotFoundError checks whether an error indicates that an enum bundle could not be found by name.
func IsEnumBundleNotFoundError(err error) bool {
	return errors.Is(err, errEnumBundleNotFound)
}

// CreateEnumBundle creates a new enum bundle.
func (c *Client) CreateEnumBundle(ctx context.Context, bundle EnumBundle) (*EnumBundle, error) {
	return createAndDecode(ctx, c, bundle, createConfig{
		PathFormat: enumBundleFieldsPath,
		HostURL:    c.HostURL,
		APIPath:    enumBundlesAPIPath,
		Fields:     enumBundleFieldsParam,
		ErrMarshal: errMarshalEnumBundle,
		ErrCreate:  "failed to create create enum bundle request: %w",
		ErrFetch:   "failed to create enum bundle: %w",
		ErrDecode:  "failed to unmarshal created enum bundle: %w",
	})
}

// UpdateEnumBundle updates a specific enum bundle by ID.
func (c *Client) UpdateEnumBundle(ctx context.Context, id string, bundle EnumBundle) (*EnumBundle, error) {
	return updateAndDecode(ctx, c, id, bundle, updateConfig{
		PathFormat: enumBundleByIDPath,
		HostURL:    c.HostURL,
		APIPath:    enumBundlesAPIPath,
		Fields:     enumBundleFieldsParam,
		ErrMarshal: errMarshalEnumBundle,
		ErrCreate:  "failed to create update enum bundle request: %w",
		ErrFetch:   "failed to update enum bundle: %w",
		ErrDecode:  "failed to unmarshal updated enum bundle: %w",
	})
}

// DeleteEnumBundle deletes an enum bundle by ID.
// While YouTrack still reports the bundle as in use, shortly after the last
// field using it was removed, the delete is retried.
func (c *Client) DeleteEnumBundle(ctx context.Context, id string) error {
	return deleteBundleByID(ctx, c, id, deleteConfig{
		APIPath:   enumBundlesAPIPath,
		ErrCreate: "failed to create delete enum bundle request: %w",
		ErrFetch:  "failed to delete enum bundle: %w",
	})
}

var enumBundleValues = bundleValueEndpoint{APIPath: enumBundlesAPIPath, Fields: enumBundleValueFields, Kind: "enum"}

// AddEnumBundleValue adds a value to an enum bundle.
func (c *Client) AddEnumBundleValue(ctx context.Context, bundleID string, value EnumBundleElement) (*EnumBundleElement, error) {
	return addBundleValue[EnumBundleElement](ctx, c, enumBundleValues, bundleID, value)
}

// ReplaceEnumBundleValue replaces the fields of one value in an enum bundle.
func (c *Client) ReplaceEnumBundleValue(ctx context.Context, bundleID, valueID string, value EnumBundleValueUpdate) (*EnumBundleElement, error) {
	return replaceBundleValue[EnumBundleElement](ctx, c, enumBundleValues, bundleID, valueID, value)
}

// DeleteEnumBundleValue removes one value from an enum bundle, waiting until
// YouTrack no longer lists it. A value that is already gone is treated as
// deleted.
func (c *Client) DeleteEnumBundleValue(ctx context.Context, bundleID, valueID string) error {
	return deleteBundleValue(ctx, c, enumBundleValues, bundleID, valueID, func(ctx context.Context) (bool, error) {
		bundle, err := c.GetEnumBundleByID(ctx, bundleID)
		if err != nil {
			return false, err
		}
		return bundleListsValue(bundle.Values, valueID, func(v EnumBundleElement) string { return v.ID }), nil
	})
}
