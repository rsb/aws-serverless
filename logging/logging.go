package logging

import (
	"context"

	"github.com/rsb/failure"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	invocationLogger ctxKey = "invocationLogger"
)

func NewLogger(service, version string) (*zap.SugaredLogger, error) {
	config := zap.NewProductionConfig()
	config.OutputPaths = []string{"stdout"}
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	config.DisableStacktrace = true
	config.InitialFields = map[string]interface{}{
		"service":         service,
		"service-version": version,
	}

	log, err := config.Build()
	if err != nil {
		return nil, failure.ToConfig(err, "[zap.NewProductionConfig()] config.Build failed")
	}

	return log.Sugar(), nil
}

// contextKey is an internal type used for context keys to restrict access
// to the context values via the various Get methods.
type ctxKey string

// SetInvocationLogger sets the request-scoped logger in the context.
func SetInvocationLogger(ctx context.Context, l *zap.SugaredLogger) context.Context {
	return context.WithValue(ctx, invocationLogger, l)
}

// GetInvocationLogger gets the invocation-scoped logger from the context. Defaults to nil if not set.
func GetInvocationLogger(ctx context.Context) *zap.SugaredLogger {
	val := ctx.Value(invocationLogger)
	logger, ok := val.(*zap.SugaredLogger)
	if !ok {
		logger = nil
	}
	return logger
}
