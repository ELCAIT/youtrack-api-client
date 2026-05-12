package youtrack

// User represents a user in YouTrack
type User struct {
	ID       string `json:"id"`
	Login    string `json:"login"`
	FullName string `json:"fullName"`
	Email    string `json:"email"`
}

// NestedGroup represents a nested group in YouTrack
type NestedGroup struct {
	ID                   string        `json:"id"`
	Description          string        `json:"description"`
	ParentGroup          *NestedGroup  `json:"parentGroup"`
	SubGroups            []NestedGroup `json:"subGroups"`
	OwnUsers             []User        `json:"ownUsers"`
	RequireTwoFactorAuth bool          `json:"requireTwoFactorAuthentication"`
	Viewers              []Holder      `json:"viewers"`
	Updaters             []Holder      `json:"updaters"`
	AutoJoin             bool          `json:"autoJoin"`
	AutoJoinDomain       string        `json:"autoJoinDomain"`
	Name                 string        `json:"name"`
	RingId               string        `json:"ringId"`
	Icon                 string        `json:"icon"`
	AllUsersGroup        bool          `json:"allUsersGroup"`
	UsersCount           int64         `json:"usersCount"`
	Users                []User        `json:"users"`
}
