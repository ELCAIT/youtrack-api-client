package youtrack

// RolesResponse wraps a list of roles.
type RolesResponse struct {
	Roles []Role `json:"roles"`
}

// PermissionsResponse wraps a list of permissions.
type PermissionsResponse struct {
	Permissions []Permission `json:"permissions"`
}

// Role represents a YouTrack role.
type Role struct {
	Id string `json:"id,omitempty"`
	// Key is not part of the YouTrack Role entity and is retained only for
	// backward compatibility. Verified against YouTrack 2026.2: the field is
	// absent from the OpenAPI schema, is silently ignored when sent on a
	// create, and is never returned on a read — even when requested
	// explicitly, because YouTrack drops unknown names from a fields query
	// instead of rejecting them. Setting it has no effect; do not rely on it.
	// Permission.Key, by contrast, is a real field and is populated.
	//
	// Deprecated: Role has no key in the YouTrack API. This field will be
	// removed in the next major version.
	Key         string       `json:"key,omitempty"`
	Name        string       `json:"name,omitempty"`
	Description string       `json:"description,omitempty"`
	Permissions []Permission `json:"permissions,omitempty"`

	// Immutable reports whether the role is built into YouTrack and cannot be
	// modified or deleted. A reconciling caller should check it before
	// attempting an update: the built-in roles (for example "Contributor")
	// report true, and writes to them fail. It is read-only and is never sent
	// on a create or update.
	Immutable bool `json:"immutable,omitempty"`
}

// Permission represents a YouTrack permission.
type Permission struct {
	Id   string `json:"id,omitempty"`
	Key  string `json:"key,omitempty"`
	Name string `json:"name,omitempty"`
}

// PermissionGraphEntry represents a permission and its implied/dependent links.
type PermissionGraphEntry struct {
	Id                   string                 `json:"id,omitempty"`
	Key                  string                 `json:"key,omitempty"`
	Name                 string                 `json:"name,omitempty"`
	ImpliedPermissions   []PermissionGraphEntry `json:"impliedPermissions,omitempty"`
	DependentPermissions []PermissionGraphEntry `json:"dependentPermissions,omitempty"`
}

// AssignedRole represents a role assigned to a user or group within a specific access scope.
type AssignedRole struct {
	Id     string `json:"id,omitempty"`
	Role   Role   `json:"role,omitempty"`
	Scope  Scope  `json:"scope,omitempty"`
	Holder Holder `json:"holder,omitempty"`
	Type   string `json:"$type,omitempty"`
}

// Scope represents the AccessScope a role is assigned in. Type ($type) discriminates
// between "GlobalScope", "OrganizationScope", and "ProjectScope". Project is only
// set (and only meaningful) when Type is "ProjectScope".
type Scope struct {
	Id      string   `json:"id,omitempty"`
	Project *Project `json:"project,omitempty"`
	Type    string   `json:"$type,omitempty"`
}

// Holder represents the user or group a role is assigned to.
type Holder struct {
	Id          string `json:"id,omitempty"`
	RingID      string `json:"ringId,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Login       string `json:"login,omitempty"`
	Type        string `json:"$type,omitempty"`
}
