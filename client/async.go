package youtrack

import (
	"context"
	"reflect"
)

// readBackAfterWrite performs the read that follows an acknowledged write,
// polling until the value the server reports reflects the change.
//
// Several YouTrack settings endpoints apply a write asynchronously: they return
// success, then take a moment to make the new value visible to reads. Reading
// once, immediately, can therefore return the previous state and make a
// successful update look like it did nothing — which for a reconciling caller
// looks like permanent drift and triggers an endless write loop.
//
// read is retried until settled reports that its result reflects the write, or
// until the poll budget runs out. The final read is returned either way: the
// write was already acknowledged by the server, so reporting an error after a
// slow convergence would be wrong more often than it would be right. A
// cancelled context is reported as an error, because then the result is
// genuinely unknown.
func readBackAfterWrite[T any](
	ctx context.Context,
	read func(context.Context) (T, error),
	settled func(T) bool,
) (T, error) {
	var (
		last    T
		lastErr error
	)

	attempt := func(attemptCtx context.Context) bool {
		last, lastErr = read(attemptCtx)
		if lastErr != nil {
			// A failed read is not a reason to keep polling: the caller needs
			// to see the error rather than wait out the budget.
			return true
		}

		return settled(last)
	}

	if err := awaitAsyncProcessing(ctx, attempt); err != nil {
		var zero T

		return zero, err
	}

	return last, lastErr
}

// readBackEqual is readBackAfterWrite for the common case where the write is
// visible once the server reports a value equal to the one that was sent.
//
// want is the value the caller wrote, projected by field so that only the
// fields the write actually controls are compared; server-populated fields such
// as IDs and computed state are excluded by the projection rather than by this
// helper.
func readBackEqual[T any, K comparable](
	ctx context.Context,
	read func(context.Context) (T, error),
	project func(T) K,
	want K,
) (T, error) {
	return readBackAfterWrite(ctx, read, func(got T) bool {
		return project(got) == want
	})
}

// readBackDeepEqual is readBackEqual for a projection that is not comparable
// with ==, such as one containing a slice.
func readBackDeepEqual[T any, K any](
	ctx context.Context,
	read func(context.Context) (T, error),
	project func(T) K,
	want K,
) (T, error) {
	return readBackAfterWrite(ctx, read, func(got T) bool {
		return reflect.DeepEqual(project(got), want)
	})
}
