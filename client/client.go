package youtrack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	// HTTP client configuration
	defaultHTTPTimeout = 10 * time.Second

	// HTTP methods
	httpMethodGet    = "GET"
	httpMethodPost   = "POST"
	httpMethodPut    = "PUT"
	httpMethodDelete = "DELETE"

	// HTTP headers
	headerAuthorization = "Authorization"
	headerContentType   = "Content-Type"
	headerUserAgent     = "User-Agent"
	headerRetryAfter    = "Retry-After"

	// Content types
	contentTypeJSON = "application/json"

	// Authorization format
	authBearerFormat = "Bearer %s"

	// defaultUserAgent identifies this client to the YouTrack instance, so an
	// operator's traffic can be told apart from the UI's in server-side logs.
	defaultUserAgent = "elcait-youtrack-api-client/" + Version

	// Async processing polling configuration. Some YouTrack settings endpoints
	// apply a write asynchronously, so a read issued immediately afterwards can
	// still report the previous value. Writes poll the read-back until it
	// converges rather than sleeping for a fixed duration.
	asyncPollInterval = 100 * time.Millisecond
	asyncPollTimeout  = 5 * time.Second

	// defaultMaxIdleConnsPerHost raises Go's default of 2. A reconcile loop
	// talks to exactly one host, so a low per-host cap forces connection churn.
	defaultMaxIdleConnsPerHost = 10
)

// Version is the client version reported in the User-Agent header.
const Version = "1.7.0"

// HTTPError represents an HTTP error response.
//
// YouTrack reports failures as a JSON body carrying an error code and a
// human-readable description; both are parsed into Error and ErrorDescription
// when present, with the raw payload kept in Body. RetryAfter carries the
// Retry-After header when the server sent one, which a caller can surface
// directly as a requeue delay.
type HTTPError struct {
	StatusCode int
	Body       []byte
	Message    string

	// Error is YouTrack's machine-readable error code, empty when the response
	// body was not a recognisable YouTrack error payload.
	ErrorCode string
	// ErrorDescription is YouTrack's human-readable explanation, empty when the
	// response body was not a recognisable YouTrack error payload.
	ErrorDescription string
	// RetryAfter is the delay requested by the server's Retry-After header.
	// Zero when the header was absent or unparsable.
	RetryAfter time.Duration
}

// Error implements the error interface.
func (e *HTTPError) Error() string {
	return e.Message
}

// youtrackErrorBody is the error payload shape used by both the YouTrack and
// Hub REST APIs.
type youtrackErrorBody struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
	// Some endpoints report the description under "error_developer_message"
	// instead, so both are accepted.
	ErrorDeveloperMessage string `json:"error_developer_message"`
}

// newHTTPError builds an HTTPError from a failed response, parsing YouTrack's
// error payload and Retry-After header when they are present. A body that is
// not a YouTrack error payload is not an error in itself: the raw bytes are
// still carried on the returned value.
func newHTTPError(res *http.Response, body []byte) *HTTPError {
	httpErr := &HTTPError{
		StatusCode: res.StatusCode,
		Body:       body,
		Message:    fmt.Sprintf("unexpected status code %d: %s", res.StatusCode, body),
		RetryAfter: parseRetryAfter(res.Header.Get(headerRetryAfter)),
	}

	var parsed youtrackErrorBody
	if err := json.Unmarshal(body, &parsed); err != nil {
		return httpErr
	}

	httpErr.ErrorCode = parsed.Error
	httpErr.ErrorDescription = parsed.ErrorDescription
	if httpErr.ErrorDescription == "" {
		httpErr.ErrorDescription = parsed.ErrorDeveloperMessage
	}

	if httpErr.ErrorDescription != "" {
		httpErr.Message = fmt.Sprintf("unexpected status code %d: %s", res.StatusCode, httpErr.ErrorDescription)
	}

	return httpErr
}

// parseRetryAfter interprets a Retry-After header, which HTTP allows to be
// either a delay in seconds or an absolute HTTP date. It returns zero when the
// header is absent or malformed, and never returns a negative duration.
func parseRetryAfter(value string) time.Duration {
	if value == "" {
		return 0
	}

	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
		if seconds <= 0 {
			return 0
		}

		return time.Duration(seconds) * time.Second
	}

	if when, err := http.ParseTime(value); err == nil {
		if delay := time.Until(when); delay > 0 {
			return delay
		}
	}

	return 0
}

// IsNotFoundError checks if an error is a 404 Not Found error.
func IsNotFoundError(err error) bool {
	return hasStatusCode(err, http.StatusNotFound)
}

// IsConflict reports whether err is a 409 Conflict response. YouTrack uses it
// when a write collides with the current state of the entity, so a reconciling
// caller should re-read and retry rather than treat the write as terminal.
func IsConflict(err error) bool {
	return hasStatusCode(err, http.StatusConflict)
}

// IsAlreadyExists reports whether err indicates that the entity a create tried
// to add is already present, which it detects as a 409 Conflict.
//
// Reconcile loops are at-least-once: a create whose response is lost on the way
// back is retried, and the second attempt must not be mistaken for a genuine
// failure. This predicate names that case at the call site, where IsConflict
// would read as if the caller were handling a lost update.
//
// Caveat: which endpoints answer a duplicate create with a 409 rather than a
// 400 has not been verified against a live instance for every resource in this
// client, and YouTrack is not consistent here. Treat a false result as
// inconclusive rather than as proof the entity is absent: confirm with a read
// before creating.
func IsAlreadyExists(err error) bool {
	return IsConflict(err)
}

// IsUnauthorized reports whether err is a 401 response, which means the token
// is missing, malformed, or expired. Retrying will not help until it is
// replaced.
func IsUnauthorized(err error) bool {
	return hasStatusCode(err, http.StatusUnauthorized)
}

// IsForbidden reports whether err is a 403 response, which means the token
// authenticated but lacks the permission the call needs. Retrying will not help
// until the permission is granted.
func IsForbidden(err error) bool {
	return hasStatusCode(err, http.StatusForbidden)
}

// IsRateLimited reports whether err is a 429 response. When it is, HTTPError's
// RetryAfter carries the delay the server asked for, if it sent one; see
// RetryAfter for the value to wait before retrying.
func IsRateLimited(err error) bool {
	return hasStatusCode(err, http.StatusTooManyRequests)
}

// IsRetryable reports whether err describes a failure that may succeed if the
// same call is made again later. It is true for rate limiting (429), for the
// transient server-side statuses (500, 502, 503, 504), and for transport-level
// failures such as timeouts, connection refusals, and cancelled contexts, which
// leave the outcome of the call unknown.
//
// It is false for every response that reflects a durable problem with the
// request itself — 400, 401, 403, 404, 405, 409, 422 — where retrying the
// identical call only repeats the failure. A controller should surface those to
// the user (for example on the resource's status) instead of requeuing.
//
// The 409 case is deliberate: a conflict means the caller's view of the entity
// is stale, so the fix is to re-read and build a new request, not to resend
// this one. Test for it with IsConflict.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		switch httpErr.StatusCode {
		case http.StatusTooManyRequests,
			http.StatusInternalServerError,
			http.StatusBadGateway,
			http.StatusServiceUnavailable,
			http.StatusGatewayTimeout:
			return true
		default:
			return false
		}
	}

	// Not an HTTP response at all: the request never completed, so whether the
	// call took effect is unknown and retrying is the safe course.
	return true
}

// RetryAfter returns the delay the server asked the caller to wait before
// retrying, and whether one was given. A controller can pass the result
// straight to a requeue-after result; when ok is false it should fall back to
// its own backoff.
func RetryAfter(err error) (time.Duration, bool) {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) && httpErr.RetryAfter > 0 {
		return httpErr.RetryAfter, true
	}

	return 0, false
}

// hasStatusCode reports whether err carries an HTTPError with the given status.
func hasStatusCode(err error, status int) bool {
	var httpErr *HTTPError

	return errors.As(err, &httpErr) && httpErr.StatusCode == status
}

// Client holds the HTTP client and configuration for YouTrack API.
//
// A Client is safe for concurrent use by multiple goroutines once constructed,
// provided its exported fields are not mutated afterwards. Configure it through
// the Option arguments to NewClient rather than by assigning to the fields, so
// that a client shared across reconcile workers stays race-free.
type Client struct {
	HostURL    string
	HTTPClient *http.Client
	Token      string

	userAgent string
	logger    *slog.Logger
}

// Option configures a Client. Options are applied by NewClient in the order
// given.
type Option func(*Client)

// WithHTTPClient replaces the HTTP client used for every request. Use it to
// supply a custom transport — a private CA bundle for a self-hosted YouTrack, a
// proxy, or instrumentation. The supplied client's Timeout is respected as
// given, so set one unless the transport or the caller's context bounds the
// request.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.HTTPClient = httpClient
		}
	}
}

// WithTimeout sets the per-request timeout on the client's HTTP client. A
// context deadline shorter than this still wins, so a caller can always tighten
// it per call.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		if timeout > 0 {
			c.HTTPClient.Timeout = timeout
		}
	}
}

// WithUserAgent overrides the User-Agent header sent with every request.
// Identifying the calling operator by name makes its traffic distinguishable in
// YouTrack's server logs, which matters when several tools share an instance.
func WithUserAgent(userAgent string) Option {
	return func(c *Client) {
		if userAgent != "" {
			c.userAgent = userAgent
		}
	}
}

// WithLogger attaches a structured logger, which the client uses to record one
// debug line per request (method, URL, status, duration) and one warning per
// failure. Pass a logr-backed slog.Logger to route this into a
// controller-runtime log stream:
//
//	youtrack.WithLogger(slog.New(logr.ToSlogHandler(mgr.GetLogger())))
//
// The client never logs the token or request and response bodies, because both
// routinely carry secrets.
func WithLogger(logger *slog.Logger) Option {
	return func(c *Client) {
		if logger != nil {
			c.logger = logger
		}
	}
}

// NewClient creates a new YouTrack API client.
//
// host is the base URL of the instance, for example
// "https://youtrack.example.com"; a trailing slash is accepted and normalised
// away. It returns an error when host is empty, is not a valid absolute URL, or
// when token is empty, so that a misconfigured client fails at construction
// rather than on its first request.
func NewClient(host, token string, opts ...Option) (*Client, error) {
	normalizedHost, err := normalizeHostURL(host)
	if err != nil {
		return nil, err
	}

	if token == "" {
		return nil, errors.New("youtrack: token must not be empty")
	}

	c := &Client{
		HTTPClient: &http.Client{
			Timeout:   defaultHTTPTimeout,
			Transport: defaultTransport(),
		},
		HostURL:   normalizedHost,
		Token:     token,
		userAgent: defaultUserAgent,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

// defaultTransport clones Go's default transport and raises the per-host idle
// connection limit. A client owned by an operator is long-lived and talks to a
// single host, so the stock cap of 2 idle connections per host would force
// repeated TCP and TLS handshakes under concurrent reconciles.
func defaultTransport() http.RoundTripper {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return http.DefaultTransport
	}

	cloned := transport.Clone()
	cloned.MaxIdleConnsPerHost = defaultMaxIdleConnsPerHost

	return cloned
}

// doRequest executes an HTTP request with authentication.
func (c *Client) doRequest(req *http.Request) ([]byte, error) {
	req.Header.Set(headerAuthorization, fmt.Sprintf(authBearerFormat, c.Token))
	req.Header.Set(headerContentType, contentTypeJSON)
	if c.userAgent != "" {
		req.Header.Set(headerUserAgent, c.userAgent)
	}

	started := time.Now()

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		c.logFailure(req, started, err)

		return nil, fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		c.logFailure(req, started, err)

		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	c.logRequest(req, res, started)

	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		return nil, newHTTPError(res, body)
	}

	return body, nil
}

// logRequest records a completed request. Bodies are deliberately omitted:
// they carry tokens, passwords, and app settings.
func (c *Client) logRequest(req *http.Request, res *http.Response, started time.Time) {
	if c.logger == nil {
		return
	}

	c.logger.LogAttrs(req.Context(), slog.LevelDebug, "youtrack request",
		slog.String("method", req.Method),
		slog.String("url", req.URL.Redacted()),
		slog.Int("status", res.StatusCode),
		slog.Duration("duration", time.Since(started)),
	)
}

// logFailure records a request that never produced a response.
func (c *Client) logFailure(req *http.Request, started time.Time, err error) {
	if c.logger == nil {
		return
	}

	c.logger.LogAttrs(req.Context(), slog.LevelWarn, "youtrack request failed",
		slog.String("method", req.Method),
		slog.String("url", req.URL.Redacted()),
		slog.Duration("duration", time.Since(started)),
		slog.String("error", err.Error()),
	)
}

// awaitAsyncProcessing waits for a write to become visible to a subsequent
// read. Several YouTrack settings endpoints acknowledge a write before applying
// it, so a read issued immediately afterwards can still report the previous
// value.
//
// It polls settled until that reports true, giving up after asyncPollTimeout,
// and returns as soon as ctx is done. Callers treat a timeout as success and
// return whatever the final read produced: the write itself was already
// acknowledged, so failing here would report an error for a change that has
// almost certainly been applied.
//
// This replaces an unconditional sleep, which blocked every caller for a fixed
// delay whether or not the write had landed, and ignored cancellation — a
// combination that stalls a controller's worker goroutines and delays its
// shutdown.
func awaitAsyncProcessing(ctx context.Context, settled func(context.Context) bool) error {
	if settled(ctx) {
		return nil
	}

	deadline := time.NewTimer(asyncPollTimeout)
	defer deadline.Stop()

	ticker := time.NewTicker(asyncPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return nil
		case <-ticker.C:
			if settled(ctx) {
				return nil
			}
		}
	}
}
