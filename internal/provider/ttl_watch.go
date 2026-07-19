package provider

import (
	"context"
	"fmt"
	"time"
)

// TipFunc returns the current chain tip slot.
type TipFunc func() (uint64, error)

// WithTTLCancel cancels the returned context with a descriptive cause when the
// chain tip reaches or passes ttlSlot. ttlSlot <= 0 disables the watcher.
func WithTTLCancel(parent context.Context, tip TipFunc, ttlSlot int64) (context.Context, context.CancelCauseFunc) {
	ctx, cancel := context.WithCancelCause(parent)
	if ttlSlot <= 0 || tip == nil {
		return ctx, cancel
	}
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		check := func() bool {
			slot, err := tip()
			if err != nil {
				return false
			}
			if slot >= uint64(ttlSlot) {
				cancel(fmt.Errorf(
					"transaction validity expired: tip slot %d >= ttl %d; rebuild and resubmit the unsigned tx",
					slot, ttlSlot,
				))
				return true
			}
			return false
		}
		if check() {
			return
		}
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if check() {
					return
				}
			}
		}
	}()
	return ctx, cancel
}
