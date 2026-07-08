package youtrack

// RolesResponse wraps a list of roles.
type RolesResponse struct {
	Roles []Role `json:"roles"`
}

// AssignedRolesResponse wraps a list of assigned roles.
type AssignedRolesResponse struct {
	AssignedRoles []AssignedRoles `json:"assignedRoles"`
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

// AssignedRoles represents a role assigned to a user or group.
type AssignedRoles struct {
	Id     string `json:"id,omitempty"`
	Role   Role   `json:"role,omitempty"`
	Scope  Scope  `json:"scope,omitempty"`
	Holder Holder `json:"holder,omitempty"`
	Type   string `json:"$type,omitempty"`
}

type Scope struct {
	Id   string `json:"id,omitempty"`
	Type string `json:"$type,omitempty"`
}

type Holder struct {
	Id          string `json:"id,omitempty"`
	RingID      string `json:"ringId,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Login       string `json:"login,omitempty"`
	Type        string `json:"$type,omitempty"`
}
