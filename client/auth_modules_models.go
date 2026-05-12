package youtrack

const (
	// oauth2AuthModuleType is the type discriminator for the Hub API.
	oauth2AuthModuleType = "Oauth2authmoduleJSON"
)

// OAuth2AuthModule represents a Hub OAuth 2.0 authentication module.
type OAuth2AuthModule struct {
	ID                     string `json:"id,omitempty"`
	Type                   string `json:"type,omitempty"`
	Name                   string `json:"name,omitempty"`
	Disabled               bool   `json:"disabled"`
	ClientID               string `json:"clientId,omitempty"`
	ClientSecret           string `json:"clientSecret,omitempty"` //nolint:gosec // G117: field name reflects the OAuth2 protocol term, not a hardcoded secret
	RedirectURI            string `json:"redirectUri,omitempty"`
	IconURL                string `json:"iconUrl,omitempty"`
	ExtensionGrantType     string `json:"extensionGrantType,omitempty"`
	ServerURL              string `json:"serverUrl,omitempty"`
	ConnectionTimeout      int    `json:"connectionTimeout,omitempty"`
	ReadTimeout            int    `json:"readTimeout,omitempty"`
	BackgroundSyncEnabled  bool   `json:"backgroundSyncEnabled"`
	SyncInterval           string `json:"syncInterval,omitempty"`
	AllowedCreateNewUsers  bool   `json:"allowedCreateNewUsers"`
	Scope                  string `json:"scope,omitempty"`
	TokenURL               string `json:"tokenUrl,omitempty"`
	FormClientAuth         bool   `json:"formClientAuth"`
	UserInfoURL            string `json:"userInfoUrl,omitempty"`
	IDPLogoutURL           string `json:"idpLogoutUrl,omitempty"`
	UserIDPath             string `json:"userIdPath,omitempty"`
	UserEmailURL           string `json:"userEmailUrl,omitempty"`
	UserAvatarURL          string `json:"userAvatarUrl,omitempty"`
	UserEmailPath          string `json:"userEmailPath,omitempty"`
	UserEmailVerifiedPath  string `json:"userEmailVerifiedPath,omitempty"`
	UserNamePath           string `json:"userNamePath,omitempty"`
	FullNamePath           string `json:"fullNamePath,omitempty"`
	UserPictureIDPath      string `json:"userPictureIdPath,omitempty"`
	UserPictureURLPattern  string `json:"userPictureUrlPattern,omitempty"`
	EmailVerifiedByDefault bool   `json:"emailVerifiedByDefault"`
	UserGroupsPath         string `json:"userGroupsPath,omitempty"`
	IsDefault              bool   `json:"default"`
}

// AuthModulesListResponse represents the paged list response from the Hub auth modules API.
type AuthModulesListResponse struct {
	AuthModules []OAuth2AuthModule `json:"authmodules"`
}
