package sls

import (
	"context"
	"strings"

	"github.com/aws/aws-lambda-go/lambdacontext"
	"go.uber.org/zap"
)

const ()

// InvocationLogger gets a logger with fields initialized for a particular invocation.
func InvocationLogger(ctx context.Context, l *zap.SugaredLogger, invType InvokeTrigger) *zap.SugaredLogger {
	if ctx == nil {
		return l
	}

	lCtx, ok := lambdacontext.FromContext(ctx)
	if !ok {
		return l
	}

	l.With(
		"function_name", lambdacontext.FunctionName,
		"function_version", lambdacontext.FunctionVersion,
		"request_id", lCtx.AwsRequestID,
		"amzn_trace_id", GetTraceID(ctx),
		"invoke_function_arn", lCtx.InvokedFunctionArn,
		"invocation_type", invType,
	)
	return l
}

// GetTraceID gets the trace id from the context. The value is in the format of
// "Root=1-5abc5ca4-f07ab5d0a2c2b2f0730acb08;Parent=200406d9510e71a3;Sampled=0"
// and the root trace id is parsed out and returned.
func GetTraceID(ctx context.Context) string {
	val := ctx.Value(amznTraceIDCtxKey)
	id, ok := val.(string)
	if !ok {
		id = ""
	}

	parts := strings.Split(id, ";")
	kv := make(map[string]string)
	for _, part := range parts {
		sub := strings.Split(part, "=")
		if len(sub) == 2 {
			kv[sub[0]] = sub[1]
		}
	}

	return kv["Root"]
}
