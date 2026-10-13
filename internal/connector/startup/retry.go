package startup

import (
	"context"
	"log/slog"
	"time"
)

const (
	initialRetryDelay = time.Second
	maximumRetryDelay = time.Minute
)

func retryWithBackoff(ctx context.Context, log *slog.Logger, operation string, attempt func() error) error {
	return retryWithBackoffAndWait(ctx, log, operation, attempt, waitForRetry)
}

func retryWithBackoffAndWait(
	ctx context.Context,
	log *slog.Logger,
	operation string,
	attempt func() error,
	wait func(context.Context, time.Duration) error,
) error {
	delay := initialRetryDelay
	failures := 0

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := attempt(); err == nil {
			return nil
		} else {
			failures++
			log.Warn("connector operation failed; retrying",
				"operation", operation,
				"failure", failures,
				"retry_in", delay,
				"error", err,
			)
		}

		if err := wait(ctx, delay); err != nil {
			return err
		}
		if delay < maximumRetryDelay/2 {
			delay *= 2
		} else {
			delay = maximumRetryDelay
		}
	}
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
