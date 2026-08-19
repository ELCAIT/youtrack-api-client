package youtrack

import (
	"encoding/json"
	"fmt"
	"strings"
)

// AppAuthor is the author reported for a YouTrack app.
//
// The apps API is not part of the officially documented YouTrack REST API (see
// the package-level note in apps.go), so the JSON shape of this field is not
// guaranteed and has no published schema. To keep a response-shape change from
// breaking every apps call, it unmarshals from either a plain JSON string or a
// JSON object exposing a "name" (or "login") key, and always marshals back as a
// plain string.
type AppAuthor string

// UnmarshalJSON accepts either a JSON string or an object with a name/login key.
func (a *AppAuthor) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		*a = ""
		return nil
	}

	var asString string
	if err := json.Unmarshal(data, &asString); err == nil {
		*a = AppAuthor(asString)
		return nil
	}

	var asObject struct {
		Name  string `json:"name"`
		Login string `json:"login"`
	}
	if err := json.Unmarshal(data, &asObject); err != nil {
		return fmt.Errorf("failed to unmarshal app author: %w", err)
	}

	if asObject.Name != "" {
		*a = AppAuthor(asObject.Name)
	} else {
		*a = AppAuthor(asObject.Login)
	}

	return nil
}

// MarshalJSON always emits the author as a plain JSON string.
func (a AppAuthor) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(string(a))
	if err != nil {
		return nil, fmt.Errorf("failed to marshal app author: %w", err)
	}

	return data, nil
}

// App represents an app installed on a YouTrack instance.
//
// Enabled reflects the instance-wide state of the app as reported by the API.
// This client exposes it read-only: no write endpoint for the instance-wide
// flag has been confirmed with JetBrains, so toggling an app for a project is
// done through its AppUsage entries instead (see EnableAppForProject).
type App struct {
	ID      string    `json:"id,omitempty"`
	Name    string    `json:"name,omitempty"`
	Enabled bool      `json:"enabled"`
	AppType string    `json:"type,omitempty"`
	Version string    `json:"version,omitempty"`
	Author  AppAuthor `json:"author,omitempty"`
}

// AppUsage represents the attachment of an app to a single project: whether the
// app is available in that project at all, and whether it is currently enabled
// there. An app with no AppUsage for a project is not attached to it.
type AppUsage struct {
	ID      string   `json:"id,omitempty"`
	Enabled bool     `json:"enabled"`
	Project *Project `json:"project,omitempty"`
}

// appUsageAttachPayload is the request body for attaching an app to a project.
type appUsageAttachPayload struct {
	Project *Project `json:"project"`
}

// appUsageEnabledPayload is the request body for toggling an existing usage.
type appUsageEnabledPayload struct {
	Enabled bool `json:"enabled"`
}
