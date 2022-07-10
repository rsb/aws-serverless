package cog

import (
	"context"

	"github.com/aws/aws-lambda-go/events"
)

const (
	apiVersion      ctxKey = "apiVersion"
	invocationEvent ctxKey = "event"

	isAdminUserKey           ctxKey = "isAdminUser"
	isCustomerServiceUserKey ctxKey = "isCustomerServiceUser"
	requestLogger            ctxKey = "requestLogger"
	rolesKey                 ctxKey = "roles"
	userIDKey                ctxKey = "userID"
)

// contextKey is an internal type used for context keys to restrict access
// to the context values via the various Get methods.
type ctxKey string

// setEvent sets the apigw event in the context.
func setPreSignupEvent(ctx context.Context, e events.CognitoEventUserPoolsPreSignup) context.Context {
	return context.WithValue(ctx, invocationEvent, e)
}

// SetAPIVersion sets the api version in the context.
func SetAPIVersion(ctx context.Context, version string) context.Context {
	return context.WithValue(ctx, apiVersion, version)
}
