package youtrack

// AuthModule identifies a Hub authentication module.
//
// Hub reports the module both on itself and on every user detail it produced. The
// name is what an operator recognises ("TrustID Int"), but it is instance-specific
// and renameable, so code that must address a module reliably uses the id.
type AuthModule struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
}

// UserDetail is one authentication identity attached to a Hub user.
//
// A user has one detail per authentication module they are known to: a Hub-local
// password login, plus one for every external provider. For an external module the
// Identifier is the account's id at that provider -- the subject of an OIDC/OAuth2
// login -- which makes it the key for reconciling that provider's events against
// YouTrack accounts.
//
// Hub normally writes a detail on first login. It can also be created ahead of time,
// which is what provisioning from an upstream directory requires: without it an
// account that has never logged in cannot be found by its upstream id.
type UserDetail struct {
	ID string `json:"id,omitempty"`
	// Type is Hub's polymorphic discriminator (for example "Oauth2detailsJSON").
	// It is required when creating a detail, since it selects the subtype.
	Type string `json:"type,omitempty"`
	// Identifier is the user's id at the external provider.
	Identifier string `json:"identifier,omitempty"`
	// AuthModule selects the module the detail belongs to. Only the id is needed
	// when creating one.
	AuthModule *AuthModule `json:"authModule,omitempty"`
	// AuthModuleName is returned by Hub for convenience and ignored on create.
	AuthModuleName string `json:"authModuleName,omitempty"`
	UserName       string `json:"userName,omitempty"`
	FullName       string `json:"fullName,omitempty"`
}

// HubUser is a user as the Hub REST API represents it.
//
// This is a different projection from User, which models the YouTrack-side account:
// Hub is where the authentication details live, so a lookup by external identity
// returns this shape.
type HubUser struct {
	ID      string       `json:"id,omitempty"`
	Login   string       `json:"login,omitempty"`
	Name    string       `json:"name,omitempty"`
	Banned  bool         `json:"banned,omitempty"`
	Details []UserDetail `json:"details,omitempty"`
}

// Oauth2DetailsType is the Hub discriminator for a detail produced by an OAuth2 or
// OIDC authentication module. Keycloak-backed modules report this type.
const Oauth2DetailsType = "Oauth2detailsJSON"
