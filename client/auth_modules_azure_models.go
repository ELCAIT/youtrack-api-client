package youtrack

const (
	// azureAuthModuleType is the type discriminator for the Hub API.
	azureAuthModuleType = "AzureauthmoduleJSON"
)

// AzureAuthModule represents a Hub Microsoft Entra ID (formerly Azure AD)
// authentication module.
//
// RedirectURI, IconURL, ExtensionGrantType, and Tenant intentionally omit
// `omitempty` so marshaling this struct as an update payload always sends an
// explicit empty value rather than dropping the key — Hub's authmodule
// update endpoint leaves omitted keys untouched, so dropping the key
// silently keeps the previous value instead of clearing it (confirmed for
// Tenant: clearing it to "" switches the module back to multi-tenant
// "common" login, and Hub only applies that when the key is present).
//
// ServerURL and SyncInterval keep `omitempty` and are never populated by
// this client: ServerURL is derived by Hub from Tenant and Hub rejects the
// request outright if it's present but empty ("url should be valid url but
// was"), and SyncInterval has the same "field unknown" rejection on an
// explicit empty value that OAuth2AuthModule has.
type AzureAuthModule struct {
	ID                     string `json:"id,omitempty"`
	Type                   string `json:"type,omitempty"`
	Name                   string `json:"name,omitempty"`
	Disabled               bool   `json:"disabled"`
	ClientID               string `json:"clientId,omitempty"`
	ClientSecret           string `json:"clientSecret,omitempty"` //nolint:gosec // G117: field name reflects the OAuth2 protocol term, not a hardcoded secret
	RedirectURI            string `json:"redirectUri"`
	IconURL                string `json:"iconUrl"`
	ExtensionGrantType     string `json:"extensionGrantType"`
	ServerURL              string `json:"serverUrl,omitempty"`
	ConnectionTimeout      int    `json:"connectionTimeout"`
	ReadTimeout            int    `json:"readTimeout"`
	BackgroundSyncEnabled  bool   `json:"backgroundSyncEnabled"`
	SyncInterval           string `json:"syncInterval,omitempty"`
	AllowedCreateNewUsers  bool   `json:"allowedCreateNewUsers"`
	Tenant                 string `json:"tenant"`
	RequestGroupPermission bool   `json:"requestGroupPermission"`
	RequestIDToken         bool   `json:"requestIdToken"`
	IsDefault              bool   `json:"default"`
}
