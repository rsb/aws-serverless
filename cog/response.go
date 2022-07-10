package cog

import (
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/rsb/failure"
)

func FailureToGatewayResponse(err error) events.APIGatewayProxyResponse {
	var resp = events.APIGatewayProxyResponse{}
	switch {
	case failure.IsPanic(err):
		resp.StatusCode = http.StatusBadGateway
	case failure.IsTimeout(err):
		resp.StatusCode = http.StatusGatewayTimeout
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
