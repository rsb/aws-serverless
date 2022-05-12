package apigw

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rsb/sls"
	"github.com/rsb/sls/logging"

	"go.uber.org/zap"

	"github.com/aws/aws-lambda-go/events"
	"github.com/rsb/failure"
)

const (
	JsonMediaType               = "application/json"
	PlainTextMediaType          = "text/plain"
	HeaderAccessCtrlAllowOrigin = "Access-Control-Allow-Origin"
	HeaderContentType           = "Content-Type"
	WildCardMediaType           = "*/*"
)

type RestHandlerConfig struct {
	sls.TimeoutConfig
	AppName string `conf:"env:APP_NAME"`
}

type Handler interface {
	Run(ctx context.Context, e events.APIGatewayProxyRequest) (*Success, error)
}

type RestRunner struct {
	feature Handler
	logger  *zap.SugaredLogger
	timeout sls.TimeoutCapturing
}

func (h *RestRunner) Handle(ctx context.Context, req events.APIGatewayProxyRequest) (out interface{}, err error) {
	var success *Success
	var resp events.APIGatewayProxyResponse

	start := time.Now()
	ctx = InitializeRequestContext(ctx, req)

	logger := RequestLogger(ctx, h.logger, req)
	ctx = logging.SetInvocationLogger(ctx, logger)

	handlerFn := func() (out interface{}, err error) {
		success, err = h.feature.Run(ctx, req)
		return out, err
	}

	_, err = h.timeout.WithTimeConstraint(ctx, handlerFn)
	elapsed := time.Since(start) / time.Millisecond

	if err != nil {
		resp, err = ProcessFailure(err, logger, req, elapsed)
		if err != nil {
			return resp, failure.Wrap(err, "ProcessFailure failed (%s)", req.RequestContext.RequestID)
		}
		return resp, nil
	}

	resp, err = ProcessSuccess(success, logger, req)
	if err != nil {
		return resp, failure.Wrap(err, "ProcessSuccess failed (%s)", req.RequestContext.RequestID)
	}

	logging.GetInvocationLogger(ctx).With(
		"status", resp.StatusCode,
		"elapsed_ms", elapsed,
	).Info(req.Path)

	return resp, nil
}

func ProcessFailure(err error, l *zap.SugaredLogger, req events.APIGatewayProxyRequest, elapsed time.Duration) (events.APIGatewayProxyResponse, error) {
	resp := FailureToGatewayResponse(err)
	l.With(
		"status", resp.StatusCode,
		"body", req.Body,
		"elapsed_ms", elapsed,
	)

	if failure.IsTimeout(err) {
		l.With("timeout", true)
	}

	if failure.IsPanic(err) {
		l.With("panic", true)
	}

	if resp.StatusCode >= http.StatusInternalServerError {
		l.Error(err.Error())
	} else {
		l.Warn(err.Error())
	}

	if req.Headers != nil {
		resp.Headers = req.Headers
	}
	resp.Headers[HeaderAccessCtrlAllowOrigin] = "*"
	resp.Headers[HeaderContentType] = JsonMediaType

	msg := http.StatusText(resp.StatusCode)
	fmt.Println(msg)
	failed := ErrorResponse{
		ID:      req.RequestContext.RequestID,
		Status:  resp.StatusCode,
		Message: msg,
	}

	if failure.IsRestAPI(err) {
		if value, ok := failure.RestMessage(err); ok {
			failed.Message = value
		}

		if fields, ok := failure.GetInvalidFields(err); ok {
			failed.Fields = fields
		}

		if code, ok := failure.RestStatusCode(err); ok {
			failed.Status = code
		}
	}

	body, err := json.Marshal(failed)
	if err != nil {
		return resp, failure.ToSystem(err, "json.Marshal failed (%v)", failed)
	}
	resp.Body = string(body)

	return resp, nil
}

func ProcessSuccess(s *Success, l *zap.SugaredLogger, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	var out events.APIGatewayProxyResponse
	var err error

	if s == nil {
		return out, failure.System("Success struct can not be nil")
	}

	// Feature decided to take full control of the response going back to apigw
	if s.SuccessFn != nil {
		out, err = s.SuccessFn(s.StatusCode, s.Body, l, req)
		if err != nil {
			return out, failure.Wrap(err, "s.SuccessFn failed(%d, %v)", s.StatusCode, s.Body)
		}

		return out, nil
	}

	code := http.StatusOK
	if s.StatusCode >= http.StatusOK && s.StatusCode < http.StatusBadRequest {
		code = s.StatusCode
	}
	headers := map[string]string{}
	if len(s.Headers) > 0 {
		headers = s.Headers
	}

	if _, ok := headers[HeaderAccessCtrlAllowOrigin]; !ok {
		headers[HeaderAccessCtrlAllowOrigin] = "*"
	}

	good := events.APIGatewayProxyResponse{
		StatusCode: code,
		Headers:    headers,
	}

	if good.StatusCode == http.StatusNoContent {
		return good, nil
	}

	if body, ok := s.Body.(string); ok {
		good.Body = body
		good.Headers[HeaderContentType] = PlainTextMediaType
		return good, nil
	}

	good.Headers[HeaderContentType] = JsonMediaType

	return good, nil
}
