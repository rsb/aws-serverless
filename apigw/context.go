package apigw

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

func InitializeRequestContext(ctx context.Context, event events.APIGatewayProxyRequest) context.Context {
	ctx = setEvent(ctx, event)
	return ctx
}

// setEvent sets the apigw event in the context.
func setEvent(ctx context.Context, event events.APIGatewayProxyRequest) context.Context {
	return context.WithValue(ctx, invocationEvent, event)
}

// SetAPIVersion sets the api version in the context.
func SetAPIVersion(ctx context.Context, version string) context.Context {
	return context.WithValue(ctx, apiVersion, version)
}

// GetAPIVersion gets the api version from the context. Defaults to an empty string if not set.
func GetAPIVersion(ctx context.Context) string {
	val := ctx.Value(apiVersion)
	version, ok := val.(string)
	if !ok {
		version = ""
	}
	return version
}

// GetEvent gets the apigw event from the context. Defaults to a new empty instance if not set.
func GetEvent(ctx context.Context) events.APIGatewayProxyRequest {
	val := ctx.Value(invocationEvent)
	event, ok := val.(events.APIGatewayProxyRequest)
	if !ok {
		event = events.APIGatewayProxyRequest{}
	}
	return event
}

// setIsCustomerServiceUser sets a boolean value on whether the user is a customer service user in the context.
func setIsCustomerServiceUser(ctx context.Context, isCS bool) context.Context {
	return context.WithValue(ctx, isCustomerServiceUserKey, isCS)
}

// GetIsCustomerServiceUser gets a boolean value on whether the user is a customer service user from the context. Defaults to false if not set.
func GetIsCustomerServiceUser(ctx context.Context) bool {
	val := ctx.Value(isCustomerServiceUserKey)
	isCS, ok := val.(bool)
	if !ok {
		isCS = false
	}
	return isCS
}

// GetUserID gets the user id from the context. Defaults to an empty string if not set.
func GetUserID(ctx context.Context) string {
	val := ctx.Value(userIDKey)
	id, ok := val.(string)
	if !ok {
		id = ""
	}
	return id
}

// setRoles sets the users roles in the context.
func setRoles(ctx context.Context, roles string) context.Context {
	return context.WithValue(ctx, rolesKey, roles)
}

// GetRoles gets the users roles from the context. Defaults to an empty string if not set.
func GetRoles(ctx context.Context) string {
	val := ctx.Value(rolesKey)
	roles, ok := val.(string)
	if !ok {
		roles = ""
	}
	return roles
}

// func setUserIDTraceLabel(ctx context.Context) {
// 	userID := GetUserID(ctx)
// 	if userID != "" {
// 		epsagon.Label("user_id", userID)
// 	}
// }
