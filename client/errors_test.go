package youtrack

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func statusErr(code int) error {
	return &HTTPError{StatusCode: code, Message: fmt.Sprintf("status %d", code)}
}

func TestErrorClassification(t *testing.T) {
	tests := []struct {
		name          string
		err           error
		retryable     bool
		notFound      bool
		conflict      bool
		forbidden     bool
		unauthorized  bool
		rateLimited   bool
		alreadyExists bool
	}{
		{name: "400 is terminal", err: statusErr(http.StatusBadRequest)},
		{name: "401 unauthorized", err: statusErr(http.StatusUnauthorized), unauthorized: true},
		{name: "403 forbidden", err: statusErr(http.StatusForbidden), forbidden: true},
		{name: "404 not found", err: statusErr(http.StatusNotFound), notFound: true},
		{name: "409 conflict", err: statusErr(http.StatusConflict), conflict: true, alreadyExists: true},
		{name: "422 is terminal", err: statusErr(http.StatusUnprocessableEntity)},
		{name: "429 retryable", err: statusErr(http.StatusTooManyRequests), retryable: true, rateLimited: true},
		{name: "500 retryable", err: statusErr(http.StatusInternalServerError), retryable: true},
		{name: "502 retryable", err: statusErr(http.StatusBadGateway), retryable: true},
		{name: "503 retryable", err: statusErr(http.StatusServiceUnavailable), retryable: true},
		{name: "504 retryable", err: statusErr(http.StatusGatewayTimeout), retryable: true},
		{name: "transport failure retryable", err: errors.New("dial tcp: connection refused"), retryable: true},
		{name: "context deadline retryable", err: context.DeadlineExceeded, retryable: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryable(tt.err); got != tt.retryable {
				t.Errorf("IsRetryable = %v, want %v", got, tt.retryable)
			}
			if got := IsNotFoundError(tt.err); got != tt.notFound {
				t.Errorf("IsNotFoundError = %v, want %v", got, tt.notFound)
			}
			if got := IsConflict(tt.err); got != tt.conflict {
				t.Errorf("IsConflict = %v, want %v", got, tt.conflict)
			}
			if got := IsForbidden(tt.err); got != tt.forbidden {
				t.Errorf("IsForbidden = %v, want %v", got, tt.forbidden)
			}
			if got := IsUnauthorized(tt.err); got != tt.unauthorized {
				t.Errorf("IsUnauthorized = %v, want %v", got, tt.unauthorized)
			}
			if got := IsRateLimited(tt.err); got != tt.rateLimited {
				t.Errorf("IsRateLimited = %v, want %v", got, tt.rateLimited)
			}
			if got := IsAlreadyExists(tt.err); got != tt.alreadyExists {
				t.Errorf("IsAlreadyExists = %v, want %v", got, tt.alreadyExists)
			}
		})
	}
}

func TestIsRetryableNilIsFalse(t *testing.T) {
	if IsRetryable(nil) {
		t.Error("a nil error is not retryable")
	}
}

func TestClassificationSeesThroughWrapping(t *testing.T) {
	wrapped := fmt.Errorf("failed to update project: %w", statusErr(http.StatusConflict))

	if !IsConflict(wrapped) {
		t.Error("IsConflict should unwrap")
	}
	if IsRetryable(wrapped) {
		t.Error("a wrapped 409 is still terminal")
	}
}

func TestNotFoundPredicatesAgree(t *testing.T) {
	// A 404 response and the ErrNotFound sentinel must both satisfy IsNotFound,
	// which is the predicate callers are told to prefer.
	if !IsNotFound(statusErr(http.StatusNotFound)) {
		t.Error("a 404 should satisfy IsNotFound")
	}
	if !IsNotFound(notFoundf("project with name %q", "demo")) {
		t.Error("the sentinel should satisfy IsNotFound")
	}
	if IsNotFound(statusErr(http.StatusInternalServerError)) {
		t.Error("a 500 must not read as absence")
	}
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  time.Duration
	}{
		{name: "empty", value: "", want: 0},
		{name: "seconds", value: "12", want: 12 * time.Second},
		{name: "seconds with spaces", value: " 12 ", want: 12 * time.Second},
		{name: "zero", value: "0", want: 0},
		{name: "negative", value: "-5", want: 0},
		{name: "garbage", value: "soon", want: 0},
		{name: "past http date", value: "Mon, 02 Jan 2006 15:04:05 GMT", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseRetryAfter(tt.value); got != tt.want {
				t.Errorf("parseRetryAfter(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestParseRetryAfterHTTPDate(t *testing.T) {
	future := time.Now().UTC().Add(30 * time.Second).Format(http.TimeFormat)

	got := parseRetryAfter(future)
	if got <= 0 || got > 31*time.Second {
		t.Errorf("parseRetryAfter(future date) = %v, want roughly 30s", got)
	}
}

func TestRetryAfterAbsent(t *testing.T) {
	if _, ok := RetryAfter(statusErr(http.StatusTooManyRequests)); ok {
		t.Error("no Retry-After header means no delay is reported")
	}
	if _, ok := RetryAfter(errors.New("boom")); ok {
		t.Error("a non-HTTP error carries no Retry-After")
	}
}
