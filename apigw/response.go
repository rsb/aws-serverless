package apigw

import (
	"encoding/json"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/rsb/failure"
	"go.uber.org/zap"
)

// ErrorResponse is the type used when returning an error from a handler.
// Message is the error message
// ID is the request id that can be used to find request details in a logging repository.
type ErrorResponse struct {
	Message string            `json:"message" `
	Fields  map[string]string `json:"fields,omitempty"`
	ID      string            `json:"id"`
	Status  int               `json:"status"`
}

type Success struct {
	StatusCode int
	Headers    map[string]string

	Body      interface{}
	SuccessFn func(code int, body interface{}, l *zap.SugaredLogger, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error)
}

func (s *Success) AddHeader(k, v string) {
	if s.Headers == nil {
		s.Headers = map[string]string{}
	}

	s.Headers[k] = v
}

func (s *Success) AddHeaders(h map[string]string) {
	for k, v := range h {
		s.AddHeader(k, v)
	}
}

func OK(body interface{}) Success {
	return Success{
		StatusCode: http.StatusOK,
		Body:       body,
	}
}

func NoContent() Success {
	return Success{StatusCode: http.StatusNoContent}
}

func BadRequest(err error, body interface{}) events.APIGatewayProxyResponse {
	b, err := json.Marshal(body)
	if err != nil {
		// what to do here
	}
	resp := events.APIGatewayProxyResponse{
		StatusCode: http.StatusBadRequest,
		Body:       string(b),
	}

	return resp
}

func FailureToGatewayResponse(err error) events.APIGatewayProxyResponse {
	var resp = events.APIGatewayProxyResponse{}
	switch {
	case failure.IsPanic(err):
		resp.StatusCode = http.StatusBadGateway
	case failure.IsTimeout(err):
		resp.StatusCode = http.StatusGatewayTimeout
	case failure.IsRestAPI(err):
		code, ok := failure.RestStatusCode(err)
		if !ok {
			code = http.StatusInternalServerError
		}

		// build body for request
		resp.StatusCode = code
	case failure.IsNotFound(err):
		resp.StatusCode = http.StatusNotFound
	case failure.IsNotAuthorized(err):
		resp.StatusCode = http.StatusUnauthorized
	case failure.IsNotAuthenticated(err):
		resp.StatusCode = http.StatusForbidden
	default:
		resp.StatusCode = http.StatusInternalServerError
	}

	resp.Body = http.StatusText(resp.StatusCode)
	return resp
}
