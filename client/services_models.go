package youtrack

// Service represents a Hub service: an external application registered for
// authentication/authorization (e.g. an OAuth client), as exposed via the
// Hub REST API's "services" endpoint.
//
// HomeURL, Description, RedirectURIs, and BaseURLs intentionally omit
// `omitempty` so marshaling this struct as an update payload always sends an
// explicit empty value rather than dropping the key — Hub's services update
// endpoint leaves omitted keys untouched, so dropping the key silently keeps
// the previous value instead of clearing it (confirmed against a live Hub
// instance: an update body of {"description":""} clears the description,
// while omitting the key entirely leaves it untouched; the same holds for
// RedirectURIs/BaseURLs with an explicit empty array vs. an omitted/null
// key).
//
// Key, ApplicationName, Vendor, and Version keep `omitempty`: ApplicationName,
// Vendor, and Version can only be set at creation — Hub rejects an update
// that changes them with "Only service itself can update its vendor,
// applicationName, version, and releaseDate" — and Key defaults to Name when
// omitted at creation, so there is never a need to send an intentional empty
// value for these fields.
type Service struct {
	ID              string   `json:"id,omitempty"`
	Type            string   `json:"type,omitempty"`
	Name            string   `json:"name,omitempty"`
	Key             string   `json:"key,omitempty"`
	HomeURL         string   `json:"homeUrl"`
	ApplicationName string   `json:"applicationName,omitempty"`
	Description     string   `json:"description"`
	Vendor          string   `json:"vendor,omitempty"`
	Version         string   `json:"version,omitempty"`
	RedirectURIs    []string `json:"redirectUris"`
	BaseURLs        []string `json:"baseUrls"`
	Trusted         bool     `json:"trusted"`
	ConsentRequired bool     `json:"consentRequired"`
	Secret          string   `json:"secret,omitempty"` //nolint:gosec // G117: field name reflects the Hub service credential term, not a hardcoded secret
}

// ServicesResponse wraps the paginated Hub services list response.
type ServicesResponse struct {
	Services []Service `json:"services"`
}
