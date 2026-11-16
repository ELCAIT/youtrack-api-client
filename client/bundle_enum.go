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
	enumBundleValueFields = "id,name,localizedName,description,archived,ordinal," + fieldStyleFields + ",$type"
	enumBundleFieldsParam = "fields=id,name,isUpdateable,values(" + enumBundleValueFields + "),$type"
	enumBundlePageSize    = 100
	errMarshalEnumBundle  = "failed to marshal enum bundle: %w"
)

var errEnumBundleNotFound = fmt.Errorf("enum bundle %w", ErrNotFound)

// EnumBundleElement represents a single value inside an enum bundle.
//
// Color is only sent when set. A value added without one gets the default style, see FieldStyle.
type EnumBundleElement struct {
	ID            string      `json:"id,omitempty"`
	Name          string      `json:"name,omitempty"`
	LocalizedName string      `json:"localizedName,omitempty"`
	Description   string      `json:"description,omitempty"`
	Archived      bool        `json:"archived,omitempty"`
	Ordinal       int         `json:"ordinal,omitempty"`
	Color         *FieldStyle `json:"color,omitempty"`
	Type          string      `json:"$type,omitempty"`
}

// EnumBundleValueUpdate is the request body for ReplaceEnumBundleValue. It
// replaces every field it carries: a nil LocalizedName or Description is sent
// as null and clears it, and Archived is always sent. Ordinal and Color are
// only sent when set: a value replaced without a Color keeps the one it has,
// and a Color is reset by setting the style DefaultFieldStyleID, see FieldStyle.
type EnumBundleValueUpdate struct {
	Name          string      `json:"name"`
	LocalizedName *string     `json:"localizedName"`
	Description   *string     `json:"description"`
	Archived      bool        `json:"archived"`
	Ordinal       *int        `json:"ordinal,omitempty"`
	Color         *FieldStyle `json:"color,omitempty"`
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

// CreateEnumBundle creates a new enum bundle. The Color of a value whose ID is not in the palette, a number
// from DefaultFieldStyleID (0) to MaxFieldStyleID (35), is refused, see FieldStyle.
func (c *Client) CreateEnumBundle(ctx context.Context, bundle EnumBundle) (*EnumBundle, error) {
	if err := validateNewEnumValues(bundle.Values); err != nil {
		return nil, err
	}

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

// validateNewEnumValues checks the Color of the values that have no ID, the ones YouTrack would create. The
// existing values are not checked: YouTrack ignores the changes to them, and what it returns for a value can be
// sent back to it. The error names the value at fault.
func validateNewEnumValues(values []EnumBundleElement) error {
	for _, value := range values {
		if value.ID != "" {
			continue
		}
		if err := validateFieldStyle(value.Color); err != nil {
			return fmt.Errorf("enum bundle value %q: %w", value.Name, err)
		}
	}

	return nil
}

// UpdateEnumBundle updates a specific enum bundle by ID. The Color of a new value, one without an ID, is
// checked like in CreateEnumBundle; the existing values are not.
func (c *Client) UpdateEnumBundle(ctx context.Context, id string, bundle EnumBundle) (*EnumBundle, error) {
	if err := validateNewEnumValues(bundle.Values); err != nil {
		return nil, err
	}

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

// AddEnumBundleValue adds a value to an enum bundle. A Color whose ID is not in the palette, a number from
// DefaultFieldStyleID (0) to MaxFieldStyleID (35), is refused, see FieldStyle.
func (c *Client) AddEnumBundleValue(ctx context.Context, bundleID string, value EnumBundleElement) (*EnumBundleElement, error) {
	if err := validateFieldStyle(value.Color); err != nil {
		return nil, err
	}

	return addBundleValue[EnumBundleElement](ctx, c, enumBundleValues, bundleID, value)
}

// ReplaceEnumBundleValue replaces the fields of one value in an enum bundle. A Color whose ID is not in the
// palette, a number from DefaultFieldStyleID (0) to MaxFieldStyleID (35), is refused, see FieldStyle.
func (c *Client) ReplaceEnumBundleValue(ctx context.Context, bundleID, valueID string, value EnumBundleValueUpdate) (*EnumBundleElement, error) {
	if err := validateFieldStyle(value.Color); err != nil {
		return nil, err
	}

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
