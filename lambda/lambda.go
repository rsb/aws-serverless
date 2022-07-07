package lambda

import (
	"context"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/lambda/types"

	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsLambda "github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/rsb/failure"
	"github.com/rsb/sls"
	"go.uber.org/zap"
)

const (
	amznTraceIDCtxKey          = "x-amzn-trace-id"
	DefaultOutputName          = "bootstrap"
	DefaultBinaryZipName       = "deployment.zip"
	DefaultLambdaInvokeType    = "RequestResponse"
	DefaultLambdaInvokeLogType = "Tail"
)

func NewClientWithConfig(cfg aws.Config) *Client {
	api := awsLambda.NewFromConfig(cfg)
	return NewClient(api)
}

type AdapterAPI interface {
	UpdateFunctionCode(ctx context.Context, params *awsLambda.UpdateFunctionCodeInput, optFns ...func(*awsLambda.Options)) (*awsLambda.UpdateFunctionCodeOutput, error)
	UpdateFunctionConfiguration(ctx context.Context, params *awsLambda.UpdateFunctionConfigurationInput, optFns ...func(*awsLambda.Options)) (*awsLambda.UpdateFunctionConfigurationOutput, error)
}

type CodePayload struct {
	DryRun        bool
	Publish       bool
	QualifiedName string
	ZipFile       []byte
}

type FeatureUpdateReport struct {
	CodeSHA256           string
	CodeSize             int64
	Description          string
	EnvError             error
	LambdaARN            string
	LambdaName           string
	LastModified         string
	LastUpdateStatus     string
	LastUpdateReason     string
	LastUpdateReasonCode string
	PackageType          string
	RevisionID           string
	Role                 string
	State                string
	StateReason          string
	StateReasonCode      string
	Timeout              int32
	Version              string
}

type FeatureSettings struct {
	QualifiedName string
	EnvVars       map[string]string
	Timeout       *int32
	Role          *string
}

type Client struct {
	api AdapterAPI
}

func NewClient(api AdapterAPI) *Client {
	return &Client{api: api}
}

func (c *Client) Compile(data sls.BuildSettings) (sls.BuildResult, error) {
	buildDir := data.BuildDir
	binName := data.BinName
	codeDir := data.CodeDir

	result := sls.BuildResult{Settings: data, ZipName: DefaultBinaryZipName}
	cmd, err := sls.NewGoBuildCmd(buildDir, binName, codeDir)
	if err != nil {
		return result, failure.Wrap(err, "NewGoBuildCmd failed for (%s,%s,%s)", buildDir, binName, codeDir)
	}

	if err = cmd.Run(); err != nil {
		return result, failure.Wrap(err, "could not build (%s) cmd.Run failed.", binName)
	}

	if data.SkipZipping {
		return result, nil
	}

	if data.ZipName != "" {
		result.ZipName = data.ZipName
	}

	binPath := filepath.Join(buildDir, binName)

	zipFile := filepath.Join(buildDir, result.ZipName)
	if err = sls.Zip(zipFile, binPath); err != nil {
		return result, failure.Wrap(err, "sls.Zip failed for (%s, %s)", zipFile, binPath)
	}

	zip, err := ioutil.ReadFile(zipFile)
	if err != nil {
		return result, failure.ToSystem(err, "ioutil.ReadFile failed")
	}
	result.ZipData = zip

	return result, nil
}

func (c *Client) UpdateCode(ctx context.Context, cp CodePayload) (*FeatureUpdateReport, error) {
	in := awsLambda.UpdateFunctionCodeInput{
		FunctionName: aws.String(cp.QualifiedName),
		ZipFile:      cp.ZipFile,
	}

	out, err := c.api.UpdateFunctionCode(ctx, &in)
	if err != nil {
		return nil, failure.ToSystem(err, "c.api.UpdateFunctionCode failed")
	}

	report := ToFeatureUpdateReportCode(out)
	return &report, nil
}

func (c *Client) UpdateConfig(ctx context.Context, fs FeatureSettings) (*FeatureUpdateReport, error) {
	in := awsLambda.UpdateFunctionConfigurationInput{
		FunctionName: aws.String(fs.QualifiedName),
		Environment:  &types.Environment{Variables: fs.EnvVars},
	}

	out, err := c.api.UpdateFunctionConfiguration(ctx, &in)
	if err != nil {
		return nil, failure.ToSystem(err, "c.api.UpdateFunction")
	}

	report := ToFeatureUpdateReportConfig(out)
	return &report, nil
}

func ToFeatureUpdateReportConfig(i *awsLambda.UpdateFunctionConfigurationOutput) FeatureUpdateReport {
	var r = FeatureUpdateReport{}

	if i == nil {
		return r
	}

	r.CodeSize = i.CodeSize
	r.LastUpdateStatus = string(i.LastUpdateStatus)
	r.LastUpdateReasonCode = string(i.LastUpdateStatusReasonCode)
	r.PackageType = string(i.PackageType)
	r.State = string(i.State)
	r.StateReasonCode = string(i.StateReasonCode)

	if i.CodeSha256 != nil {
		r.CodeSHA256 = *i.CodeSha256
	}

	if i.Description != nil {
		r.Description = *i.Description
	}

	if i.Environment != nil && i.Environment.Error != nil && i.Environment.Error.Message != nil {
		r.EnvError = failure.System(*i.Environment.Error.Message)
	}

	if i.FunctionArn != nil {
		r.LambdaARN = *i.FunctionArn
	}

	if i.FunctionName != nil {
		r.LambdaName = *i.FunctionName
	}

	if i.LastModified != nil {
		r.LastModified = *i.LastModified
	}

	if i.LastUpdateStatusReason != nil {
		r.LastUpdateReason = *i.LastUpdateStatusReason
	}

	if i.RevisionId != nil {
		r.RevisionID = *i.RevisionId
	}

	if i.Role != nil {
		r.Role = *i.Role
	}

	if i.StateReason != nil {
		r.StateReason = *i.StateReason
	}

	if i.Timeout != nil {
		r.Timeout = *i.Timeout
	}

	if i.Version != nil {
		r.Version = *i.Version
	}

	return r
}

func ToFeatureUpdateReportCode(i *awsLambda.UpdateFunctionCodeOutput) FeatureUpdateReport {
	var r = FeatureUpdateReport{}

	if i == nil {
		return r
	}

	r.CodeSize = i.CodeSize
	r.LastUpdateStatus = string(i.LastUpdateStatus)
	r.LastUpdateReasonCode = string(i.LastUpdateStatusReasonCode)
	r.PackageType = string(i.PackageType)
	r.State = string(i.State)
	r.StateReasonCode = string(i.StateReasonCode)

	if i.CodeSha256 != nil {
		r.CodeSHA256 = *i.CodeSha256
	}

	if i.Description != nil {
		r.Description = *i.Description
	}

	if i.Environment != nil && i.Environment.Error != nil && i.Environment.Error.Message != nil {
		r.EnvError = failure.System(*i.Environment.Error.Message)
	}

	if i.FunctionArn != nil {
		r.LambdaARN = *i.FunctionArn
	}

	if i.FunctionName != nil {
		r.LambdaName = *i.FunctionName
	}

	if i.LastModified != nil {
		r.LastModified = *i.LastModified
	}

	if i.LastUpdateStatusReason != nil {
		r.LastUpdateReason = *i.LastUpdateStatusReason
	}

	if i.RevisionId != nil {
		r.RevisionID = *i.RevisionId
	}

	if i.Role != nil {
		r.Role = *i.Role
	}

	if i.StateReason != nil {
		r.StateReason = *i.StateReason
	}

	if i.Timeout != nil {
		r.Timeout = *i.Timeout
	}

	if i.Version != nil {
		r.Version = *i.Version
	}

	return r
}

// InvocationLogger gets a logger with fields initialized for a particular invocation.
func InvocationLogger(ctx context.Context, l *zap.SugaredLogger, invType sls.InvokeTrigger) *zap.SugaredLogger {
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
