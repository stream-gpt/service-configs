// Package bootwait implements a bounded retry-with-backoff loop for
// boot-time dependency acquisition (postgres, NATS, ...).
//
// Motivation (tech-architect ADR-017 / TD-boot-1, Gap A): on a full host
// reboot, docker compose / k8s start every container roughly at once. This
// service restarted ~19x at boot during the 2026-07-19 reboot because
// main() called migrate.Run against postgres before postgres was accepting
// connections, hard-failing and relying on CrashLoopBackOff to eventually
// succeed — noisy, slow, and a false "crash" signal for on-call. WaitFor
// instead retries the dependency call with exponential backoff + jitter, up
// to a bounded ceiling. If the ceiling elapses, it returns the last error so
// main() can still fatal-exit exactly as before — this only smooths
// transient boot races, it does not hide a genuinely-dead dependency.
//
// This package is intentionally duplicated per-service rather than pulled
// from a shared module: the fix needs to ship now for the two proven
// crashers (chat-ingestion, configs) without requiring a lib-platform
// release. If a third service needs this, consider promoting it there.
package bootwait

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/Gen-Do/lib-observability/logger"
)

// Options configures WaitFor's retry loop. Zero-value fields fall back to
// DefaultOptions' values.
type Options struct {
	// InitialDelay is the wait before the first retry after a failed attempt.
	InitialDelay time.Duration
	// MaxDelay caps the backoff delay between retries.
	MaxDelay time.Duration
	// Multiplier grows the delay after each failed attempt (exponential backoff).
	Multiplier float64
	// Jitter is the fraction (0..1) of the current delay added as random
	// jitter, to avoid synchronized retry storms across replicas that all
	// rebooted at once.
	Jitter float64
	// Ceiling is the total time budget for retries before WaitFor gives up
	// and returns the last error. Zero means DefaultOptions().Ceiling.
	Ceiling time.Duration
}

// DefaultOptions returns the defaults used for unset fields: 500ms initial
// delay, 10s cap, 2x multiplier, 20% jitter, 3 minute ceiling.
func DefaultOptions() Options {
	return Options{
		InitialDelay: 500 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Multiplier:   2,
		Jitter:       0.2,
		Ceiling:      3 * time.Minute,
	}
}

func (o Options) withDefaults() Options {
	d := DefaultOptions()
	if o.InitialDelay <= 0 {
		o.InitialDelay = d.InitialDelay
	}
	if o.MaxDelay <= 0 {
		o.MaxDelay = d.MaxDelay
	}
	if o.Multiplier <= 1 {
		o.Multiplier = d.Multiplier
	}
	if o.Jitter < 0 {
		o.Jitter = 0
	}
	if o.Ceiling <= 0 {
		o.Ceiling = d.Ceiling
	}
	return o
}

// WaitFor calls fn until it returns nil, ctx is cancelled, or the ceiling
// elapses — whichever happens first. Every failed attempt is logged at Warn
// ("waiting for <name>…") so boot-time waits are visible in Loki instead of
// silently blocking. On ceiling exhaustion it returns the last error
// wrapped with the attempt count, so callers can fatal-exit exactly as
// before — a dependency that is still unreachable after ~2-3 minutes is
// treated as genuinely dead, not a transient boot race.
func WaitFor(ctx context.Context, log logger.Logger, name string, fn func(ctx context.Context) error, opts Options) error {
	opts = opts.withDefaults()

	deadline := time.Now().Add(opts.Ceiling)
	delay := opts.InitialDelay
	attempt := 0
	var lastErr error

	for {
		attempt++
		err := fn(ctx)
		if err == nil {
			return nil
		}
		lastErr = err

		if ctx.Err() != nil {
			return fmt.Errorf("bootwait: wait for %s: %w", name, ctx.Err())
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("bootwait: %s not ready after %d attempts over %s: %w", name, attempt, opts.Ceiling, lastErr)
		}

		wait := delay
		if opts.Jitter > 0 {
			wait += time.Duration(rand.Float64() * opts.Jitter * float64(delay))
		}
		if wait > remaining {
			wait = remaining
		}

		logCtx := log.WithFields(ctx, logger.Fields{
			"dependency": name,
			"attempt":    attempt,
			"retry_in":   wait.String(),
			"error":      err.Error(),
		})
		log.Warn(logCtx, fmt.Sprintf("waiting for %s…", name))

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("bootwait: wait for %s: %w", name, ctx.Err())
		case <-timer.C:
		}

		delay = time.Duration(float64(delay) * opts.Multiplier)
		if delay > opts.MaxDelay {
			delay = opts.MaxDelay
		}
	}
}
