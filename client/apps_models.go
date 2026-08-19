package youtrack

// AppVendor describes the publisher of an app, as reported by the apps API.
type AppVendor struct {
	Name  string `json:"name,omitempty"`
	URL   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
	Type  string `json:"$type,omitempty"`
}

// App represents an app installed on a YouTrack instance.
//
// Name is the package-style identifier (for example
// "@jetbrains/youtrack-demo-client") and is what GetAppByName matches on;
// Title is the human-readable display name shown in the UI.
//
// The field set here is the one the App entity actually exposes, verified
// against YouTrack 2026.2 (build 18194). Note that the apps API silently drops
// unknown field names from a `fields` query instead of rejecting them, so a
// field that does not exist comes back absent rather than as an error — see the
// note in apps.go about `enabled`, `type`, and `author`.
type App struct {
	ID          string     `json:"id,omitempty"`
	Name        string     `json:"name,omitempty"`
	Title       string     `json:"title,omitempty"`
	Description string     `json:"description,omitempty"`
	Version     string     `json:"version,omitempty"`
	Vendor      *AppVendor `json:"vendor,omitempty"`
	Type        string     `json:"$type,omitempty"`
}

// AppUsage represents the attachment of an app to a single project: whether the
// app is available in that project at all, and whether it is currently enabled
// there. An app with no AppUsage for a project is not attached to it.
//
// YouTrack calls this entity a ProjectAppConfiguration.
type AppUsage struct {
	ID      string   `json:"id,omitempty"`
	Enabled bool     `json:"enabled"`
	Project *Project `json:"project,omitempty"`
	Type    string   `json:"$type,omitempty"`
}

// appUsageAttachPayload is the request body for attaching an app to a project.
type appUsageAttachPayload struct {
	Project *Project `json:"project"`
}

// appUsageEnabledPayload is the request body for toggling an existing usage.
type appUsageEnabledPayload struct {
	Enabled bool `json:"enabled"`
}
