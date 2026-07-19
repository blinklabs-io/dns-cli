package provider

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestWithTTLCancelExpires(t *testing.T) {
	tip := uint64(100)
	ctx, cancel := WithTTLCancel(context.Background(), func() (uint64, error) {
		return tip, nil
	}, 50)
	defer cancel(nil)

	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("expected cancel when tip >= ttl")
	}
	cause := context.Cause(ctx)
	if cause == nil || !strings.Contains(cause.Error(), "validity expired") {
		t.Fatalf("cause=%v", cause)
	}
}

func TestWithTTLCancelDisabled(t *testing.T) {
	ctx, cancel := WithTTLCancel(context.Background(), func() (uint64, error) {
		return 999, nil
	}, 0)
	defer cancel(nil)
	select {
	case <-ctx.Done():
		t.Fatal("should not cancel when ttl disabled")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestWithTTLCancelPropagatesParent(t *testing.T) {
	parent, stop := context.WithCancel(context.Background())
	ctx, cancel := WithTTLCancel(parent, func() (uint64, error) {
		return 1, errors.New("tip unavailable")
	}, 100)
	defer cancel(nil)
	stop()
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("expected parent cancel")
	}
}
