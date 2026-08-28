package youtrack

import (
	"bytes"
	"strings"
	"sync"
)

// safeBuffer is a bytes.Buffer guarded by a mutex, so a test can read what a
// logger wrote without racing the goroutine that wrote it.
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

// contains reports whether haystack contains needle.
func contains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
