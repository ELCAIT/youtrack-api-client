package youtrack

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// normalizeHostURL validates the instance base URL and strips any trailing
// slashes, so that the rest of the client can join paths onto it without
// producing a doubled separator. It rejects a URL that is empty, unparsable,
// relative, or not http(s), so that a misconfigured host fails at construction
// rather than as a confusing request error later.
func normalizeHostURL(host string) (string, error) {
	trimmed := strings.TrimSpace(host)
	if trimmed == "" {
		return "", errors.New("youtrack: host must not be empty")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("youtrack: invalid host %q: %w", host, err)
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("youtrack: host %q must use http or https", host)
	}

	if parsed.Host == "" {
		return "", fmt.Errorf("youtrack: host %q must include a hostname", host)
	}

	return strings.TrimRight(trimmed, "/"), nil
}

// buildURL assembles a request URL from the client's host, a slash-separated
// API path, any path segments, and a query.
//
// Each segment is escaped, so an identifier taken from user input — a project
// short name or group name read from a Kubernetes resource, for example — that
// contains "/", "?", or "#" produces a correct request instead of a malformed
// URL that silently addresses a different endpoint. apiPath is treated as a
// trusted literal from this package and its separators are preserved.
func (c *Client) buildURL(apiPath string, segments []string, query url.Values) string {
	var builder strings.Builder

	builder.WriteString(strings.TrimRight(c.HostURL, "/"))
	builder.WriteString("/")
	builder.WriteString(strings.Trim(apiPath, "/"))

	for _, segment := range segments {
		builder.WriteString("/")
		builder.WriteString(url.PathEscape(segment))
	}

	if encoded := query.Encode(); encoded != "" {
		builder.WriteString("?")
		builder.WriteString(encoded)
	}

	return builder.String()
}

// fieldsQuery builds the query for a request that selects a set of fields.
func fieldsQuery(fields string) url.Values {
	values := url.Values{}
	values.Set("fields", fields)

	return values
}

// paginatedQuery builds the query for a list request, adding YouTrack's $top
// and $skip parameters only when they are set. Passing 0 for either leaves the
// server's own default in place.
func paginatedQuery(fields string, top, skip int) url.Values {
	values := fieldsQuery(fields)
	if top > 0 {
		values.Set("$top", strconv.Itoa(top))
	}
	if skip > 0 {
		values.Set("$skip", strconv.Itoa(skip))
	}

	return values
}
