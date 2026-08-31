package wait_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xieliangji/soluna-appium-client/wait"
)

func TestUntilChecksImmediatelyAndHonorsInterval(t *testing.T) {
	const interval = 5 * time.Millisecond

	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Second,
	)
	defer cancel()

	var calls atomic.Int32
	started := make(chan time.Time, 3)

	err := wait.Until(
		ctx,
		interval,
		func(context.Context) (bool, error) {
			call := calls.Add(1)
			started <- time.Now()
			return call == 3, nil
		},
	)
	if err != nil {
		t.Fatalf("Until() error = %v", err)
	}

	if got := calls.Load(); got != 3 {
		t.Fatalf("condition calls = %d, want 3", got)
	}

	first := <-started
	second := <-started
	third := <-started
	if second.Sub(first) < interval {
		t.Fatalf("first poll interval = %v, want at least %v", second.Sub(first), interval)
	}
	if third.Sub(second) < interval {
		t.Fatalf("second poll interval = %v, want at least %v", third.Sub(second), interval)
	}
}

func TestUntilReturnsConditionFailureWithoutRetry(t *testing.T) {
	wantErr := errors.New("condition failed")
	var calls atomic.Int32

	err := wait.Until(
		context.Background(),
		time.Hour,
		func(context.Context) (bool, error) {
			calls.Add(1)
			return false, wantErr
		},
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Until() error = %v, want %v", err, wantErr)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("condition calls = %d, want 1", got)
	}
}

func TestUntilStopsBeforeNextPollWhenContextEnds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	checked := make(chan struct{})
	result := make(chan error, 1)

	go func() {
		result <- wait.Until(
			ctx,
			time.Hour,
			func(context.Context) (bool, error) {
				close(checked)
				return false, nil
			},
		)
	}()

	<-checked
	cancel()

	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("Until() error = %v, want context.Canceled", err)
	}
}

func TestUntilDoesNotInvokeConditionForEndedContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var calls atomic.Int32
	err := wait.Until(
		ctx,
		time.Millisecond,
		func(context.Context) (bool, error) {
			calls.Add(1)
			return true, nil
		},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Until() error = %v, want context.Canceled", err)
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("condition calls = %d, want 0", got)
	}
}

func TestUntilRejectsInvalidArguments(t *testing.T) {
	ctx := context.Background()
	condition := func(context.Context) (bool, error) {
		return true, nil
	}

	tests := []struct {
		name string
		call func() error
	}{
		{
			name: "nil context",
			call: func() error {
				return wait.Until(nil, time.Millisecond, condition)
			},
		},
		{
			name: "non-positive interval",
			call: func() error {
				return wait.Until(ctx, 0, condition)
			},
		},
		{
			name: "negative interval",
			call: func() error {
				return wait.Until(ctx, -time.Millisecond, condition)
			},
		},
		{
			name: "nil condition",
			call: func() error {
				return wait.Until(ctx, time.Millisecond, nil)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.call(); err == nil {
				t.Fatal("Until() error = nil, want validation error")
			}
		})
	}
}

func TestUntilRejectsSuccessAfterContextDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Millisecond,
	)
	defer cancel()

	err := wait.Until(
		ctx,
		time.Millisecond,
		func(ctx context.Context) (bool, error) {
			<-ctx.Done()
			return true, nil
		},
	)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Until() error = %v, want context deadline", err)
	}
}
