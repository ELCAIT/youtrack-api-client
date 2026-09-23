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
	ownedBundleFieldsParam = "fields=id,name,isUpdateable,values(id,name,description,archived,ordinal,owner(id,login,name,$type),$type),$type"
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
