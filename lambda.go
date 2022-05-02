package sls

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/lambda"

	"github.com/rsb/failure"
)

const (
	DefaultOutputName          = "bootstrap"
	DefaultBinaryZipName       = "deployment.zip"
	DefaultInfraDir            = "infra/local"
	DefaultTerraform           = "terraform"
	DefaultLambdasDir          = "app/lambdas"
	DefaultBuildDir            = "/tmp"
	DefaultRepoRefName         = "main"
	DefaultLambdaInvokeType    = "RequestResponse"
	DefaultLambdaInvokeLogType = "Tail"

	DefaultLambdaGoFile     = "main.go"
	DefaultAppDirName       = "app"
	DefaultLambdaDirName    = "lambdas"
	DefaultInfraDirName     = "infra"
	DefaultBuildDirName     = "build"
	DefaultTerraformDirName = "terraform"

	GQLTrigger      = LambdaTrigger("gql")
	APIGWTrigger    = LambdaTrigger("apigw")
	DDBTrigger      = LambdaTrigger("ddb")
	DirectTrigger   = LambdaTrigger("direct")
	CognitoTrigger  = LambdaTrigger("cog")
	S3Trigger       = LambdaTrigger("s3")
	SNSTrigger      = LambdaTrigger("sns")
	SQSTrigger      = LambdaTrigger("sqs")
	StepFuncTrigger = LambdaTrigger("sfn")
)

type LambdaTrigger string

func (lt LambdaTrigger) String() string {
	return string(lt)
}

func (lt LambdaTrigger) IsEmpty() bool {
	return lt.String() == ""
}

func ToLambdaTrigger(s string) (LambdaTrigger, error) {
	var t LambdaTrigger
	var err error
	switch strings.ToLower(s) {
	case APIGWTrigger.String():
		t = APIGWTrigger
	case DDBTrigger.String():
		t = DDBTrigger
	case DirectTrigger.String():
		t = DirectTrigger
	case CognitoTrigger.String():
		t = CognitoTrigger
	case S3Trigger.String():
		t = S3Trigger
	case SNSTrigger.String():
		t = SNSTrigger
	case SQSTrigger.String():
		t = SQSTrigger
	case GQLTrigger.String():
		t = GQLTrigger
	case StepFuncTrigger.String():
		t = StepFuncTrigger
	default:
		err = failure.Validation("event trigger (%s) is not registered", t)
	}

	return t, err
}

type Lambda struct {
	Name          string
	QualifiedName string
	Trigger       LambdaTrigger
	BinaryName    string
	BinaryZipName string
	Conf          ConfigurableFeature
	Env           map[string]string
}

func (l Lambda) EnvNames(prefix ...string) ([]string, error) {
	names, err := l.Conf.EnvNames(prefix...)
	if err != nil {
		return nil, failure.Wrap(err, "l.Conf.EnvNames failed")
	}

	return names, nil
}

func (l Lambda) ToAWSEnv() *lambda.Environment {
	data := map[string]*string{}
	for k, v := range l.Env {
		data[k] = aws.String(v)
	}

	return &lambda.Environment{
		Variables: data,
	}
}

func (l Lambda) AddEnv(name, value string) {
	l.Env[name] = value
}

func (l Lambda) TriggerDir() string {
	return l.Trigger.String()
}

func (l Lambda) CodeDir() string {
	return filepath.Join(l.TriggerDir(), l.Name)
}

func (l Lambda) NameWithTrigger() string {
	return fmt.Sprintf("%s_%s", l.Trigger, l.Name)
}

func (l Lambda) String() string {
	return l.Name
}
