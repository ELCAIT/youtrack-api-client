package youtrack

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// HTTP client configuration
	defaultHTTPTimeout = 10 * time.Second

	// HTTP methods
	httpMethodGet    = "GET"
	httpMethodPost   = "POST"
	httpMethodDelete = "DELETE"

	// HTTP headers
	headerAuthorization = "Authorization"
	headerContentType   = "Content-Type"

	// Content types
	contentTypeJSON = "application/json"

	// Authorization format
	authBearerFormat = "Bearer %s"

	// Async processing polling configuration
	asyncPollInterval = 100 * time.Millisecond
	asyncPollTimeout  = 5 * time.Second
)

// HTTPError represents an HTTP error response.
type HTTPError struct {
	StatusCode int
	Body       []byte
	Message    string
}

// Error implements the error interface.
func (e *HTTPError) Error() string {
	return e.Message
}

// IsNotFoundError checks if an error is a 404 Not Found error.
func IsNotFoundError(err error) bool {
	var httpErr *HTTPError
	return errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound
}

// Client holds the HTTP client and configuration for YouTrack API.
type Client struct {
	HostURL    string
	HTTPClient *http.Client
	Token      string
}

// NewClient creates a new YouTrack API client.
func NewClient(host, token string) (*Client, error) {
	c := Client{
		HTTPClient: &http.Client{Timeout: defaultHTTPTimeout},
		HostURL:    host,
		Token:      token,
	}

	return &c, nil
}

// doRequest executes an HTTP request with authentication.
func (c *Client) doRequest(req *http.Request) ([]byte, error) {
	req.Header.Set(headerAuthorization, fmt.Sprintf(authBearerFormat, c.Token))
	req.Header.Set(headerContentType, contentTypeJSON)

	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute HTTP request: %w", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return nil, &HTTPError{
			StatusCode: res.StatusCode,
			Body:       body,
			Message:    fmt.Sprintf("unexpected status code %d: %s", res.StatusCode, body),
		}
	}

	return body, nil
}

// waitForAsyncProcessing waits for async operations to complete with polling.
// This is a simple delay-based approach for APIs that process updates asynchronously.
func waitForAsyncProcessing() {
	time.Sleep(asyncPollInterval * 2) // Use 2x poll interval as a conservative delay
}
