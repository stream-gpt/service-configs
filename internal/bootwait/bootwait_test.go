package bootwait

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	observability "github.com/Gen-Do/lib-observability"
)

func testLogger() *observability.Observability {
	// MustNew wires a real logrus-backed logger with no external
	// dependencies (no OTel exporter is contacted for log calls), safe to
	// use in unit tests.
	return observability.MustNew(context.Background())
}

func TestWaitFor_SucceedsAfterTransientFailures(t *testing.T) {
	obs := testLogger()
	log := obs.GetLogger()

	var calls int32
	failuresBeforeSuccess := int32(3)
	fn := func(ctx context.Context) error {
		n := atomic.AddInt32(&calls, 1)
		if n <= failuresBeforeSuccess {
			return errors.New("dependency not ready yet")
		}
		return nil
	}

	err := WaitFor(context.Background(), log, "test-dep", fn, Options{
		InitialDelay: time.Millisecond,
		MaxDelay:     5 * time.Millisecond,
		Multiplier:   2,
		Jitter:       0,
		Ceiling:      time.Second,
	})
	if err != nil {
		t.Fatalf("expected WaitFor to succeed, got error: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != failuresBeforeSuccess+1 {
		t.Fatalf("expected %d calls, got %d", failuresBeforeSuccess+1, got)
	}
}

func TestWaitFor_FatalAfterCeiling(t *testing.T) {
	obs := testLogger()
	log := obs.GetLogger()

	wantErr := errors.New("dependency permanently unreachable")
	var calls int32
	fn := func(ctx context.Context) error {
		atomic.AddInt32(&calls, 1)
		return wantErr
	}

	start := time.Now()
	err := WaitFor(context.Background(), log, "test-dep", fn, Options{
		InitialDelay: 5 * time.Millisecond,
		MaxDelay:     5 * time.Millisecond,
		Multiplier:   2,
		Jitter:       0,
		Ceiling:      30 * time.Millisecond,
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected WaitFor to return an error after the ceiling elapses")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected returned error to wrap the last error, got: %v", err)
	}
	if atomic.LoadInt32(&calls) < 2 {
		t.Fatalf("expected at least 2 attempts before giving up, got %d", calls)
	}
	// Bounded: must not run substantially longer than the configured ceiling.
	if elapsed > 500*time.Millisecond {
		t.Fatalf("WaitFor ran for %s, expected it to stop near the %s ceiling", elapsed, 30*time.Millisecond)
	}
}

func TestWaitFor_RespectsContextCancellation(t *testing.T) {
	obs := testLogger()
	log := obs.GetLogger()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var calls int32
	fn := func(ctx context.Context) error {
		atomic.AddInt32(&calls, 1)
		return errors.New("not ready")
	}

	err := WaitFor(ctx, log, "test-dep", fn, Options{
		InitialDelay: time.Millisecond,
		Ceiling:      time.Second,
	})
	if err == nil {
		t.Fatal("expected WaitFor to return an error when ctx is already cancelled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected error to wrap context.Canceled, got: %v", err)
	}
}

func TestWaitFor_SucceedsOnFirstTry(t *testing.T) {
	obs := testLogger()
	log := obs.GetLogger()

	var calls int32
	fn := func(ctx context.Context) error {
		atomic.AddInt32(&calls, 1)
		return nil
	}

	err := WaitFor(context.Background(), log, "test-dep", fn, Options{})
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected exactly 1 call, got %d", got)
	}
}
