package common

import (
	"context"
	"errors"
	"fmt"
	"github.com/urfave/cli/v2"
)

type Lifecycle interface {
	// Start starts a service. A service only fully starts once. Subsequent starts may return an error.
	// A context is provided to end the service during setup.
	// The caller should call Stop to clean up after failing to start.
	Start(ctx context.Context) error
	// Stop stops a service gracefully.
	// The provided ctx can force an accelerated shutdown,
	// but the node still has to completely stop.
	Stop(ctx context.Context) error
	// Stopped determines if the service was stopped with Stop.
	Stopped() bool
}

type LifecycleAction func(ctx *cli.Context) (Lifecycle, error)

func LifecycleCmd(fn LifecycleAction) cli.ActionFunc {
	return func(ctx *cli.Context) error {
		hostCtx := ctx.Context
		appCtx, _ := context.WithCancelCause(hostCtx)
		ctx.Context = appCtx

		appLifecycle, err := fn(ctx)
		if err != nil {
			// join errors to include context cause (nil errors are dropped)
			return errors.Join(
				fmt.Errorf("failed to setup: %w", err),
				context.Cause(appCtx),
			)
		}

		if err := appLifecycle.Start(appCtx); err != nil {
			// join errors to include context cause (nil errors are dropped)
			return errors.Join(
				fmt.Errorf("failed to start: %w", err),
				context.Cause(appCtx),
			)
		}

		// wait for app to be closed (through interrupt, or app requests to be stopped by closing the context)
		<-appCtx.Done()

		// Graceful stop context.
		// This allows the service to idle before shutdown, if halted. User may interrupt.
		stopCtx, stopCancel := context.WithCancelCause(hostCtx)

		// Execute graceful stop.
		stopErr := appLifecycle.Stop(stopCtx)
		stopCancel(nil)
		// note: Stop implementation may choose to suppress a context error,
		// if it handles it well (e.g. stop idling after a halt).
		if stopErr != nil {
			// join errors to include context cause (nil errors are dropped)
			return errors.Join(
				fmt.Errorf("failed to stop: %w", stopErr),
				context.Cause(stopCtx),
			)
		}

		return nil
	}
}
