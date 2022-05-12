package sls

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"github.com/rsb/sls/logging"

	"github.com/rsb/failure"
)

const (
	DefaultFeatureTimeout = 100
)

type TimeoutConfig struct {
	Period int `conf:"env:SLS_FEATURE_HANDLER_TIMEOUT, default:100ms"`
}

func NewTimeout(config TimeoutConfig) *Timeout {
	return &Timeout{
		period: config.Period,
	}
}

type TimeoutCapturing interface {
	WithTimeConstraint(ctx context.Context, fn ConcreteHandlerFn) (out interface{}, err error)
}

type Timeout struct {
	period int
}

func (t *Timeout) WithTimeConstraint(ctx context.Context, fn ConcreteHandlerFn) (out interface{}, err error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		logging.GetInvocationLogger(ctx).Warn("no deadline in context, timeout cannot be captured")
		out, err = fn()
		return out, err
	}

	runTime := time.Until(deadline) - (DefaultFeatureTimeout * time.Millisecond)
	completed := make(chan interface{})

	go func() {
		defer func() {
			if r := recover(); r != nil {
				err = failure.Panic("invocation")

				stack := strings.Split(string(debug.Stack()), "\n")
				logging.GetInvocationLogger(ctx).With(
					"recover", fmt.Sprintf("%v", r),
					"stack", stack,
					"panic", true,
				).Error(err)
				completed <- struct{}{}
			}
		}()

		// Execute feature handler
		out, err = fn()
		// <---------------------

		completed <- struct{}{}
	}()
	select {
	case <-completed:
	// success is a no-op
	case <-time.After(runTime):
		err = failure.Timeout("invocation timeout")
	}

	return out, err
}

type MockTimeout struct {
	ReturnFromFn bool
	Out          interface{}
	Err          error
}

func (m *MockTimeout) WithTimeConstraint(_ context.Context, fn ConcreteHandlerFn) (out interface{}, err error) {
	if fn != nil && m.ReturnFromFn {
		return fn()
	}

	return m.Out, m.Err
}
