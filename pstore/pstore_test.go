package pstore_test

import (
	"context"
	"testing"

	"github.com/rsb/sls/pstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws/aws-sdk-go-v2/aws"

	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/rsb/failure"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

func TestClient_Param_Failures(t *testing.T) {
	cases := []struct {
		name        string
		key         string
		err         error
		msg         string
		isErrorType func(err error) bool
	}{
		{
			key:         "foo",
			name:        "non 404 error returned by api",
			err:         &types.ParameterLimitExceeded{Message: aws.String("some limit error")},
			msg:         "c.api.GetParameter failed (foo)",
			isErrorType: failure.IsSystem,
		},
		{
			key:         "foo",
			name:        "404 error returned by api",
			err:         &types.ParameterNotFound{Message: aws.String("param not found")},
			msg:         "c.api.GetParameter failed (foo)",
			isErrorType: failure.IsNotFound,
		},
		{
			key:         "",
			name:        "empty key error",
			err:         nil,
			msg:         "key is empty, a non empty key is required",
			isErrorType: failure.IsSystem,
		},
	}

	ctx := context.TODO()
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			api := MockAPI{GetParamError: tt.err}
			client, err := pstore.NewClient(&api, false)
			require.NoError(t, err, "pstore.NewClient is not expected to fail")

			_, err = client.Param(ctx, tt.key)
			require.Error(t, err, "c.Param is expected to fail")
			assert.Contains(t, err.Error(), tt.msg)
			assert.True(t, tt.isErrorType(err))
		})
	}
}

func TestClient_Param_Success(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
		resp  *ssm.GetParameterOutput
	}{
		{
			name:  "some typical env var in parameter store",
			key:   "foo",
			value: "bar",
			resp: &ssm.GetParameterOutput{
				Parameter: &types.Parameter{
					Value: aws.String("bar"),
				},
			},
		},
		{
			name: "A parameter store value that is actually just an empty string",
			key:  "foo",
			resp: &ssm.GetParameterOutput{
				Parameter: &types.Parameter{
					Value: aws.String(""),
				},
			},
		},
		{
			name:  "Should never happen, but when Parameter is nil",
			key:   "foo",
			value: "",
			resp: &ssm.GetParameterOutput{
				Parameter: nil,
			},
		},
		{
			name:  "Should never happen, but if response is nil",
			key:   "foo",
			value: "",
			resp:  nil,
		},
	}

	ctx := context.TODO()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := MockAPI{GetParamResponse: tt.resp}
			c, err := pstore.NewClient(&api, false)
			require.NoError(t, err, "pstore.NewClient is not expected to fail")
			value, err := c.Param(ctx, tt.key)
			require.NoError(t, err, "c.Param is not expected to fail")
			assert.Equal(t, tt.value, value)
		})
	}
}

type MockAPI struct {
	GetParamError     error
	GetParamResponse  *ssm.GetParameterOutput
	GetPathError      error
	GetPathResponse   *ssm.GetParametersByPathOutput
	GetParamsError    error
	GetParamsResponse *ssm.GetParametersOutput
	DeleteError       error
	DeleteResponse    *ssm.DeleteParameterOutput
	PutError          error
	PutResponse       *ssm.PutParameterOutput
}

func (m *MockAPI) GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	return m.GetParamResponse, m.GetParamError
}
func (m *MockAPI) GetParameters(ctx context.Context, params *ssm.GetParametersInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersOutput, error) {
	return m.GetParamsResponse, m.GetParamsError
}
func (m *MockAPI) GetParametersByPath(ctx context.Context, params *ssm.GetParametersByPathInput, optFns ...func(*ssm.Options)) (*ssm.GetParametersByPathOutput, error) {
	return m.GetPathResponse, m.GetPathError
}
func (m *MockAPI) DeleteParameter(ctx context.Context, params *ssm.DeleteParameterInput, optFns ...func(*ssm.Options)) (*ssm.DeleteParameterOutput, error) {
	return m.DeleteResponse, m.DeleteError
}

func (m *MockAPI) PutParameter(ctx context.Context, params *ssm.PutParameterInput, optFns ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
	return m.PutResponse, m.PutError
}
