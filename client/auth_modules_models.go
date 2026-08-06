package youtrack

const (
	// oauth2AuthModuleType is the type discriminator for the Hub API.
	oauth2AuthModuleType = "Oauth2authmoduleJSON"
)

// OAuth2AuthModule represents a Hub OAuth 2.0 authentication module.
//
// Fields that a caller may legitimately need to clear back to empty/zero on
// an update (RedirectURI, IconURL, ExtensionGrantType, Scope, UserInfoURL,
// IDPLogoutURL, ConnectionTimeout, ReadTimeout, and the user*Path/URL
// claim-mapping fields) intentionally omit `omitempty` so marshaling this
// struct as an update payload always sends an explicit empty/zero value
// rather than dropping the key — Hub's authmodule update endpoint leaves
// omitted keys untouched, so dropping the key silently keeps the previous
// value instead of clearing it.
//
// SyncInterval is deliberately NOT in that list: Hub rejects the request
// outright (400 "Field ExternalAuthModule::syncInterval is unknown") when
// this field is present but empty, so it must keep `omitempty` — there is no
// way to explicitly clear it via this endpoint today.
type OAuth2AuthModule struct {
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
	Scope                  string `json:"scope"`
	TokenURL               string `json:"tokenUrl,omitempty"`
	FormClientAuth         bool   `json:"formClientAuth"`
	UserInfoURL            string `json:"userInfoUrl"`
	IDPLogoutURL           string `json:"idpLogoutUrl"`
	UserIDPath             string `json:"userIdPath,omitempty"`
	UserEmailURL           string `json:"userEmailUrl"`
	UserAvatarURL          string `json:"userAvatarUrl"`
	UserEmailPath          string `json:"userEmailPath"`
	UserEmailVerifiedPath  string `json:"userEmailVerifiedPath"`
	UserNamePath           string `json:"userNamePath"`
	FullNamePath           string `json:"fullNamePath"`
	UserPictureIDPath      string `json:"userPictureIdPath"`
	UserPictureURLPattern  string `json:"userPictureUrlPattern"`
	EmailVerifiedByDefault bool   `json:"emailVerifiedByDefault"`
	UserGroupsPath         string `json:"userGroupsPath"`
	IsDefault              bool   `json:"default"`
}

// AuthModulesListResponse represents the paged list response from the Hub auth modules API.
type AuthModulesListResponse struct {
	AuthModules []OAuth2AuthModule `json:"authmodules"`
}
