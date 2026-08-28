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
//
// Trusted, ConsentRequired, and the five OAuth flow flags
// (ClientCredentialsFlowEnabled, AuthCodeFlowEnabled, PKCERequired,
// ImplicitFlowEnabled, ResourceOwnerFlowEnabled) are plain bools, so Go's zero
// value already marshals as an explicit `false` — there is no "omitted" state
// to preserve for them. Hub defaults all of them but PKCERequired to `true`
// for a service created without specifying them; PKCERequired defaults to
// `false`. Hub does not enforce any cross-flag constraint (e.g. PKCERequired
// can be `true` while AuthCodeFlowEnabled is `false`), so this client doesn't
// validate that relationship either.
type Service struct {
	ID                           string   `json:"id,omitempty"`
	Type                         string   `json:"type,omitempty"`
	Name                         string   `json:"name,omitempty"`
	Key                          string   `json:"key,omitempty"`
	HomeURL                      string   `json:"homeUrl"`
	ApplicationName              string   `json:"applicationName,omitempty"`
	Description                  string   `json:"description"`
	Vendor                       string   `json:"vendor,omitempty"`
	Version                      string   `json:"version,omitempty"`
	RedirectURIs                 []string `json:"redirectUris"`
	BaseURLs                     []string `json:"baseUrls"`
	Trusted                      bool     `json:"trusted"`
	ConsentRequired              bool     `json:"consentRequired"`
	ClientCredentialsFlowEnabled bool     `json:"clientCredentialsFlowEnabled"`
	AuthCodeFlowEnabled          bool     `json:"authCodeFlowEnabled"`
	PKCERequired                 bool     `json:"pkceRequired"`
	ImplicitFlowEnabled          bool     `json:"implicitFlowEnabled"`
	ResourceOwnerFlowEnabled     bool     `json:"resourceOwnerFlowEnabled"`
	Secret                       string   `json:"secret,omitempty"` //nolint:gosec // G117: field name reflects the Hub service credential term, not a hardcoded secret

	// Immutable reports whether the service is built into Hub and therefore
	// cannot be modified or deleted. A reconciling caller should check it
	// before attempting an update: on a stock instance the bundled services
	// ("YouTrack", "YouTrack Administration", "YouTrack Mobile", "Konnector")
	// all report true, and writes to them fail. It is read-only and is never
	// sent on a create or update, so it keeps `omitempty`.
	Immutable bool `json:"immutable,omitempty"`
}

// ServicesResponse wraps the paginated Hub services list response.
type ServicesResponse struct {
	Services []Service `json:"services"`
}
