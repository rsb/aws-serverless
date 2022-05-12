package apigw

import (
	"context"
	"strings"

	"github.com/rsb/sls"

	"github.com/aws/aws-lambda-go/events"
	"go.uber.org/zap"
)

func RequestLogger(ctx context.Context, l *zap.SugaredLogger, e events.APIGatewayProxyRequest) *zap.SugaredLogger {
	l = sls.InvocationLogger(ctx, l, sls.APIGWProxyTrigger)

	l.With(
		"headers", e.Headers,
		"api_gateway_request_id", e.RequestContext.RequestID,
		"method", e.HTTPMethod,
		"path", e.Path,
		"resource_path", e.RequestContext.ResourcePath,
	)

	if e.RequestContext.Identity.UserArn != "" {
		l.With("user_arn", e.RequestContext.Identity.UserArn)
	}

	if params := e.PathParameters; params != nil {
		l.With("path_parameters", params)
	}

	if params := e.QueryStringParameters; params != nil {
		l.With("query_parameters", params)
	}

	headers := map[string]string{}
	for k, v := range e.Headers {
		headers[strings.ToLower(k)] = v
	}
	name, ok := headers["x-client-name"]
	if !ok {
		name = "unknown"
	}
	l.With("client_name", name)

	if version, ok := headers["x-client-version"]; ok {
		l.With("client_version", version)
	}

	if id := GetUserID(ctx); id != "" {
		l.With(
			"user_id", id,
			"roles", GetRoles(ctx),
		)
	}

	if e.RequestContext.Identity.UserArn != "" {
		l.With("user_arn", e.RequestContext.Identity.UserArn)
	}

	return l
}
