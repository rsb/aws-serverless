// Package cog is responsible for providing an aws cognito client as well as
// lambda handling front controller pattern for all microservice lambda
// features that need logging, timeout and panic handling.
package cog

import (
	"context"
	"time"

	"github.com/rsb/failure"

	"github.com/rsb/sls/logging"

	"github.com/aws/aws-lambda-go/events"
	"github.com/rsb/sls"
	"go.uber.org/zap"
)

type HandlerConfig struct {
	sls.TimeoutConfig
}

type Success struct{}

type PreSignupHandler interface {
	Run(ctx context.Context, e events.CognitoEventUserPoolsPreSignup) error
}

type PreSignupRunner struct {
	feature PreSignupHandler
	logger  *zap.SugaredLogger
	timeout sls.TimeoutCapturing
}

func (p *PreSignupRunner) Handle(ctx context.Context, e events.CognitoEventUserPoolsPreSignup) (err error) {
	ctx = setPreSignupEvent(ctx, e)

	logger := PreSignupLogger(ctx, p.logger, e)
	ctx = logging.SetInvocationLogger(ctx, logger)

	start := time.Now()
	handlerFn := func() (out interface{}, err error) {
		err = p.feature.Run(ctx, e)
		return out, err
	}

	_, err = p.timeout.WithTimeConstraint(ctx, handlerFn)
	elapsed := time.Since(start) / time.Millisecond

	logger.With("elapsed_ms", elapsed)

	if err != nil {
		switch {
		case failure.IsTimeout(err):
			logger.With("timeout", true)
		case failure.IsPanic(err):
			logger.With("panic", true)

		default:
			logger.Error("[PreSignupRunner FAILED]", err.Error())
		}
	}

	return err
}
