package cog

import (
	"context"

	"github.com/aws/aws-lambda-go/events"
	"github.com/rsb/sls"
	"go.uber.org/zap"
)

func PreSignupLogger(ctx context.Context, l *zap.SugaredLogger, e events.CognitoEventUserPoolsPreSignup) *zap.SugaredLogger {
	l = sls.InvocationLogger(ctx, l, sls.CognitoTrigger)

	l.With(
		"client_metadata", e.Request.ClientMetadata,
		"user_attributes", e.Request.UserAttributes,
	)

	return l
}
