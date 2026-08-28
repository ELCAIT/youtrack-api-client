package youtrack

import (
	"net/url"
	"testing"
)

func TestNormalizeHostURL(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		want    string
		wantErr bool
	}{
		{name: "plain", host: "https://yt.example.com", want: "https://yt.example.com"},
		{name: "trailing slash", host: "https://yt.example.com/", want: "https://yt.example.com"},
		{name: "several trailing slashes", host: "https://yt.example.com///", want: "https://yt.example.com"},
		{name: "surrounding space", host: "  https://yt.example.com  ", want: "https://yt.example.com"},
		{name: "subpath preserved", host: "https://example.com/youtrack/", want: "https://example.com/youtrack"},
		{name: "port preserved", host: "http://localhost:8080", want: "http://localhost:8080"},
		{name: "empty", host: "", wantErr: true},
		{name: "no scheme", host: "yt.example.com", wantErr: true},
		{name: "wrong scheme", host: "ftp://yt.example.com", wantErr: true},
		{name: "no hostname", host: "https:///path", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeHostURL(tt.host)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error for %q", tt.host)
				}

				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("normalizeHostURL(%q) = %q, want %q", tt.host, got, tt.want)
			}
		})
	}
}

func TestBuildURLEscapesSegments(t *testing.T) {
	c := &Client{HostURL: "https://yt.example.com"}

	tests := []struct {
		name     string
		apiPath  string
		segments []string
		query    url.Values
		want     string
	}{
		{
			name:    "no segments or query",
			apiPath: "api/admin/projects",
			want:    "https://yt.example.com/api/admin/projects",
		},
		{
			name:     "simple segment",
			apiPath:  "api/admin/projects",
			segments: []string{"0-1"},
			want:     "https://yt.example.com/api/admin/projects/0-1",
		},
		{
			name:     "segment with a slash cannot escape its position",
			apiPath:  "api/admin/projects",
			segments: []string{"evil/../../admin"},
			want:     "https://yt.example.com/api/admin/projects/evil%2F..%2F..%2Fadmin",
		},
		{
			name:     "segment with a question mark does not start a query",
			apiPath:  "api/admin/projects",
			segments: []string{"weird?fields=all"},
			want:     "https://yt.example.com/api/admin/projects/weird%3Ffields=all",
		},
		{
			name:     "segment with a space",
			apiPath:  "api/admin/projects",
			segments: []string{"My Project"},
			want:     "https://yt.example.com/api/admin/projects/My%20Project",
		},
		{
			name:     "query is appended",
			apiPath:  "api/admin/projects",
			segments: []string{"0-1"},
			query:    fieldsQuery("id,name"),
			want:     "https://yt.example.com/api/admin/projects/0-1?fields=id%2Cname",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.buildURL(tt.apiPath, tt.segments, tt.query)
			if got != tt.want {
				t.Errorf("buildURL = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildURLNormalizesSeparators(t *testing.T) {
	// A host with a trailing slash and a path with a leading one must not
	// produce a doubled separator.
	c := &Client{HostURL: "https://yt.example.com/"}

	got := c.buildURL("/api/admin/projects/", []string{"0-1"}, nil)
	want := "https://yt.example.com/api/admin/projects/0-1"
	if got != want {
		t.Errorf("buildURL = %q, want %q", got, want)
	}
}

func TestPaginatedQuery(t *testing.T) {
	tests := []struct {
		name string
		top  int
		skip int
		want string
	}{
		{name: "no pagination", top: 0, skip: 0, want: "fields=id"},
		{name: "top only", top: 50, skip: 0, want: "%24top=50&fields=id"},
		{name: "top and skip", top: 50, skip: 100, want: "%24skip=100&%24top=50&fields=id"},
		{name: "negative ignored", top: -1, skip: -1, want: "fields=id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := paginatedQuery("id", tt.top, tt.skip).Encode(); got != tt.want {
				t.Errorf("paginatedQuery = %q, want %q", got, tt.want)
			}
		})
	}
}
