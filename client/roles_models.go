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
	Id          string       `json:"id,omitempty"`
	Key         string       `json:"key,omitempty"`
	Name        string       `json:"name,omitempty"`
	Description string       `json:"description,omitempty"`
	Permissions []Permission `json:"permissions,omitempty"`
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
