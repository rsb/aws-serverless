package sls

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/service/lambda/lambdaiface"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/pkg/errors"
)

const (
	DefaultAppDirName       = "app"
	DefaultLambdaDirName    = "lambdas"
	DefaultInfraDirName     = "infra"
	DefaultBuildDirName     = "build"
	DefaultTerraformDirName = "terraform"
	APIGWTrigger            = LambdaTrigger("apigw")
	DDBTrigger              = LambdaTrigger("ddb")
	DirectTrigger           = LambdaTrigger("direct")
	CognitoTrigger          = LambdaTrigger("cognito")
	S3Trigger               = LambdaTrigger("s3")
	SNSTrigger              = LambdaTrigger("sns")
	SQSTrigger              = LambdaTrigger("sqs")
)

type LambdaTrigger string

func (lt LambdaTrigger) String() string {
	return string(lt)
}

func (lt LambdaTrigger) Empty() bool {
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
	default:
		err = errors.Errorf("event trigger (%s) is not registered", t)
	}

	return t, err
}

type Lambda struct {
	Prefix
	Service       string
	Trigger       LambdaTrigger
	BaseName      string
	BinaryName    string
	BinaryZipName string
	Env           map[string]string
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
	return filepath.Join(l.TriggerDir(), l.BaseName)
}

func (l Lambda) QualifiedName() string {
	return l.String()
}

func (l Lambda) String() string {
	name := fmt.Sprintf("%s_%s", l.Trigger, l.BaseName)
	return fmt.Sprintf("%s-%s-%s", l.Prefix.String(), l.Service, name)
}

// ServiceLayout is a collection of directories which layout where the
// required code is located in order to build and deploy aws lambdas.
//
// This layout makes the following assumptions:
// 1) App is always under the Root directory
// 2) Lambdas are always under the App directory
// 3) Infra is always under the Root directory
// 4) Build is always under the Infra directory
// 5) Terraform is always under the Infra directory
type ServiceLayout struct {
	Root      string
	App       string
	Lambdas   string
	Infra     string
	Build     string
	Terraform string
}

func DefaultServiceLayout(dir string) ServiceLayout {
	return ServiceLayout{
		Root:      dir,
		App:       DefaultAppDirName,
		Lambdas:   DefaultLambdaDirName,
		Infra:     DefaultInfraDirName,
		Build:     DefaultBuildDirName,
		Terraform: DefaultTerraformDirName,
	}
}

func (sl ServiceLayout) RootDir() string {
	return sl.Root
}

func (sl ServiceLayout) AppDir() string {
	return filepath.Join(sl.RootDir(), sl.App)
}

func (sl ServiceLayout) LambdasDir() string {
	return filepath.Join(sl.AppDir(), sl.Lambdas)
}
func (sl ServiceLayout) TriggerDir(lt LambdaTrigger) string {
	return filepath.Join(sl.LambdasDir(), lt.String())
}

func (sl ServiceLayout) InfraDir() string {
	return filepath.Join(sl.Root, sl.Infra)
}

func (sl ServiceLayout) BuildDir() string {
	return filepath.Join(sl.InfraDir(), sl.Build)
}

func (sl ServiceLayout) TerraformDir() string {
	return filepath.Join(sl.InfraDir(), sl.Terraform)
}

type Service struct {
	ServiceLayout
	Name     string
	Env      string
	API      lambdaiface.LambdaAPI
	Features map[string]Lambda
}

func NewService(name, env string, layout ServiceLayout, api lambdaiface.LambdaAPI) *Service {
	return &Service{
		ServiceLayout: layout,
		Name:          name,
		Env:           env,
		API:           api,
		Features:      map[string]Lambda{},
	}
}

func (s *Service) FeatureNames() []string {
	var names []string
	if s.Features == nil {
		return names
	}

	for name := range s.Features {
		names = append(names, name)
	}

	return names
}
