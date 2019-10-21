package clock

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTick(t *testing.T) {
	ctx, c := context.WithCancel(context.Background())
	timeout := 1500 * time.Millisecond
	jitter := 100 * time.Millisecond

	tch := make(chan time.Time)
	errch := make(chan error)
	go func() {
		errch <- Tick(ctx, tch)
		close(errch)
		close(tch)
	}()

	// Check that ticks arrive and they're about a second apart.
	var a, b time.Time
	select {
	case <-time.After(timeout):
		t.Fatal("timeout waiting for first tick")
	case err := <-errch:
		t.Fatalf("unexpected error waiting for first tick: %v", err)
	case a = <-tch:
		if delay := time.Since(a); delay > jitter {
			t.Errorf("delayed first tick: %s", delay)
		}
	}
	select {
	case <-time.After(timeout):
		t.Fatal("timeout waiting for second tick")
	case err := <-errch:
		t.Fatalf("unexpected error waiting for second tick: %v", err)
	case b = <-tch:
		if delay := time.Since(b); delay > jitter {
			t.Errorf("delayed second tick: %s", delay)
		}
	}
	if diff := b.Sub(a); diff > timeout {
		t.Errorf("too much delay between ticks: %s", diff)
	}

	// Check that missed ticks do not block the ticker.
	select {
	case <-time.After(2500 * time.Millisecond):
	case err := <-errch:
		t.Fatalf("unexpected error while sleeping: %v", err)
	}

	select {
	case <-time.After(timeout):
		t.Fatal("timeout waiting for third tick")
	case err := <-errch:
		t.Fatalf("unexpected error waiting for third tick: %v", err)
	case new := <-tch:
		if delay := time.Since(new); delay > jitter {
			t.Errorf("delayed third tick: %s", delay)
		}
	}

	// Check that cancelling the context stops the ticking.
	c()
	select {
	case <-time.After(timeout):
		t.Fatal("timeout waiting for cancel")
	case err := <-errch:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("unexpected error after cancel: %v", err)
		}
	}
}
