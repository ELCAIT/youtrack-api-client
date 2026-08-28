package youtrack

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestReadBackReturnsImmediatelyWhenSettled(t *testing.T) {
	var reads int32

	started := time.Now()
	got, err := readBackEqual(context.Background(),
		func(context.Context) (string, error) {
			atomic.AddInt32(&reads, 1)

			return "done", nil
		},
		func(s string) string { return s },
		"done")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "done" {
		t.Fatalf("got %q", got)
	}
	if n := atomic.LoadInt32(&reads); n != 1 {
		t.Errorf("expected exactly one read, got %d", n)
	}
	// The old implementation slept unconditionally; a settled read must not wait.
	if elapsed := time.Since(started); elapsed > asyncPollInterval {
		t.Errorf("a settled read took %v, expected no wait", elapsed)
	}
}

func TestReadBackPollsUntilConverged(t *testing.T) {
	var reads int32

	got, err := readBackEqual(context.Background(),
		func(context.Context) (string, error) {
			// Report the stale value twice, then the new one.
			if atomic.AddInt32(&reads, 1) < 3 {
				return "stale", nil
			}

			return "fresh", nil
		},
		func(s string) string { return s },
		"fresh")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "fresh" {
		t.Fatalf("got %q, want the converged value", got)
	}
	if n := atomic.LoadInt32(&reads); n != 3 {
		t.Errorf("expected 3 reads, got %d", n)
	}
}

func TestReadBackHonoursContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := readBackEqual(ctx,
		func(context.Context) (string, error) { return "stale", nil },
		func(s string) string { return s },
		"never")

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestReadBackStopsOnReadError(t *testing.T) {
	var reads int32
	sentinel := errors.New("read failed")

	_, err := readBackEqual(context.Background(),
		func(context.Context) (string, error) {
			atomic.AddInt32(&reads, 1)

			return "", sentinel
		},
		func(s string) string { return s },
		"whatever")

	if !errors.Is(err, sentinel) {
		t.Fatalf("expected the read error to surface, got %v", err)
	}
	if n := atomic.LoadInt32(&reads); n != 1 {
		t.Errorf("a failing read must not be retried, got %d reads", n)
	}
}

func TestReadBackReturnsLastValueOnTimeout(t *testing.T) {
	// A write that never becomes visible must still return the value the server
	// reports rather than an error: the write itself was acknowledged.
	ctx, cancel := context.WithTimeout(context.Background(), asyncPollTimeout+time.Second)
	defer cancel()

	got, err := readBackEqual(ctx,
		func(context.Context) (string, error) { return "stale", nil },
		func(s string) string { return s },
		"never-arrives")

	if err != nil {
		t.Fatalf("a slow convergence must not be an error, got %v", err)
	}
	if got != "stale" {
		t.Fatalf("got %q, want the last observed value", got)
	}
}

func TestUpdateSettingsPollsUntilServerConverges(t *testing.T) {
	// End-to-end through a real endpoint: the server reports the old locale on
	// the first read after the write, then the new one.
	var readCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == httpMethodPost {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))

			return
		}

		if atomic.AddInt32(&readCount, 1) == 1 {
			_, _ = w.Write([]byte(`{"locale":{"id":"en_US","locale":"en_US","name":"English"}}`))

			return
		}
		_, _ = w.Write([]byte(`{"locale":{"id":"de_DE","locale":"de_DE","name":"German"}}`))
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "perm:abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := c.UpdateLocaleSettings(context.Background(), LocaleSettings{
		Locale: LocaleDescriptor{ID: "de_DE", Locale: "de_DE"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Locale.Locale != "de_DE" {
		t.Fatalf("returned the stale value %q instead of polling until converged", got.Locale.Locale)
	}
	if n := atomic.LoadInt32(&readCount); n < 2 {
		t.Errorf("expected the client to re-read, got %d reads", n)
	}
}

func TestUpdateSettingsAbortsWhenContextCancelled(t *testing.T) {
	// A settings write whose read-back never converges must abort promptly when
	// the caller's context is cancelled, rather than blocking a worker.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == httpMethodPost {
			_, _ = w.Write([]byte(`{}`))

			return
		}
		_, _ = w.Write([]byte(`{"locale":{"id":"en_US","locale":"en_US"}}`))
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "perm:abc")

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	started := time.Now()
	_, err := c.UpdateLocaleSettings(ctx, LocaleSettings{
		Locale: LocaleDescriptor{ID: "de_DE", Locale: "de_DE"},
	})
	elapsed := time.Since(started)

	if err == nil {
		t.Fatal("expected the cancelled context to surface as an error")
	}
	if elapsed >= asyncPollTimeout {
		t.Errorf("cancellation took %v; it must not wait out the full poll budget", elapsed)
	}
}
