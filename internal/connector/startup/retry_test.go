package startup

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRetryWithBackoffDelaysUntilSuccess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var delays []time.Duration
	attempts := 0

	err := retryWithBackoffAndWait(
		context.Background(),
		logger,
		"test operation",
		func() error {
			attempts++
			if attempts < 4 {
				return errors.New("temporary failure")
			}
			return nil
		},
		func(_ context.Context, delay time.Duration) error {
			delays = append(delays, delay)
			return nil
		},
	)

	require.NoError(t, err)
	require.Equal(t, 4, attempts)
	require.Equal(t, []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}, delays)
}

func TestRetryWithBackoffCapsDelay(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	var delays []time.Duration
	attempts := 0

	err := retryWithBackoffAndWait(
		context.Background(),
		logger,
		"test operation",
		func() error {
			attempts++
			if attempts == 9 {
				return nil
			}
			return errors.New("temporary failure")
		},
		func(_ context.Context, delay time.Duration) error {
			delays = append(delays, delay)
			return nil
		},
	)

	require.NoError(t, err)
	require.Equal(t, []time.Duration{
		time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
		32 * time.Second,
		time.Minute,
		time.Minute,
	}, delays)
}

func TestRetryWithBackoffStopsWhenContextIsCanceled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	attempts := 0

	err := retryWithBackoffAndWait(
		ctx,
		logger,
		"test operation",
		func() error {
			attempts++
			return errors.New("temporary failure")
		},
		func(ctx context.Context, _ time.Duration) error {
			cancel()
			return ctx.Err()
		},
	)

	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, attempts)
}
