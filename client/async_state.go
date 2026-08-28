package youtrack

// The types below project an entity down to just the fields a write actually
// controls. They exist so that the read-back after an asynchronous write
// compares only what was sent, ignoring server-populated fields — IDs, computed
// state, secrets the API declines to echo back — which would otherwise never
// compare equal and would turn every write into a full poll-timeout wait.
//
// They are unexported: they are an implementation detail of the read-back, not
// part of the API surface.

// restSettingsState is the writable projection of RestSettings.
type restSettingsState struct {
	AllowAllOrigins bool
	AllowedOrigins  []string
}

// backupSettingsState is the writable projection of BackupSettings. The
// remaining fields (available disk space, backup status) are reported by the
// server and are not set by a write.
type backupSettingsState struct {
	Enabled        bool
	CronExpression string
	Location       string
	FilesToKeep    int
}

// appearanceSettingsState is the writable projection of AppearanceSettings.
type appearanceSettingsState struct {
	DateFormatID string
	TimeZoneID   string
}

// workTimeState is the writable projection of WorkTimeSettings.
type workTimeState struct {
	MinutesADay int
	WorkDays    []int
}

// serviceState is the writable projection of a Hub Service. The secret is
// deliberately excluded: Hub does not echo it back on a read, so comparing it
// would never settle.
type serviceState struct {
	Name        string
	HomeURL     string
	Description string
}

// roleState is the writable projection of a Role. Permissions are compared by
// count rather than by content because the API returns them with fully
// populated names and keys, while a write may reference them by ID alone.
type roleState struct {
	Name        string
	Description string
	Permissions int
}

// authModuleState is the writable projection shared by the OAuth 2.0 and Entra
// ID auth modules. The client secret is excluded: Hub does not return it on a
// read, so comparing it would never settle.
type authModuleState struct {
	Name      string
	Disabled  bool
	ClientID  string
	ServerURL string
}

// observedOrSkip reports the value to compare for one field of a partial
// update. It normally returns what the server reported, so the comparison waits
// for the write to become visible. When the caller did not set the field
// (want is empty) the write never touched it, so waiting for it to change would
// hang until the poll budget expired; returning want instead satisfies that
// field immediately and leaves the remaining fields to decide.
func observedOrSkip(want, observed string) string {
	if want == "" {
		return want
	}

	return observed
}

// wantedLicense returns the license key a global settings write is expected to
// produce, treating a nil License as a request to clear it.
func wantedLicense(settings GlobalSettings) string {
	if settings.License == nil {
		return ""
	}

	return settings.License.License
}
