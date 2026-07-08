package youtrack

// User represents a user in YouTrack
type User struct {
	ID       string `json:"id,omitempty"`
	RingID   string `json:"ringId,omitempty"`
	Login    string `json:"login,omitempty"`
	FullName string `json:"fullName,omitempty"`
	Email    string `json:"email,omitempty"`
	Password string `json:"password,omitempty"`
	Banned   bool   `json:"banned,omitempty"`
}

// NestedGroup represents a nested group in YouTrack
type NestedGroup struct {
	ID                   string        `json:"id,omitempty"`
	Description          string        `json:"description,omitempty"`
	ParentGroup          *NestedGroup  `json:"parentGroup,omitempty"`
	SubGroups            []NestedGroup `json:"subGroups,omitempty"`
	OwnUsers             []User        `json:"ownUsers,omitempty"`
	RequireTwoFactorAuth bool          `json:"requireTwoFactorAuthentication,omitempty"`
	Viewers              []Holder      `json:"viewers,omitempty"`
	Updaters             []Holder      `json:"updaters,omitempty"`
	AutoJoin             bool          `json:"autoJoin,omitempty"`
	AutoJoinDomain       string        `json:"autoJoinDomain,omitempty"`
	Name                 string        `json:"name,omitempty"`
	RingId               string        `json:"ringId,omitempty"`
	Icon                 string        `json:"icon,omitempty"`
	AllUsersGroup        bool          `json:"allUsersGroup,omitempty"`
	UsersCount           int64         `json:"usersCount,omitempty"`
	Users                []User        `json:"users,omitempty"`
}
