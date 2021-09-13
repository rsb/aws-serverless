package sls

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/rsb/failure"

	"github.com/aws/aws-sdk-go/service/lambda/lambdaiface"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/lambda"
)

const (
	DefaultLambdaGoFile     = "main.go"
	DefaultAppDirName       = "app"
	DefaultLambdaDirName    = "lambdas"
	DefaultInfraDirName     = "infra"
	DefaultBuildDirName     = "build"
	DefaultTerraformDirName = "terraform"
	APIGWTrigger            = LambdaTrigger("apigw")
	DDBTrigger              = LambdaTrigger("ddb")
	DirectTrigger           = LambdaTrigger("direct")
	CognitoTrigger          = LambdaTrigger("cog")
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
		err = failure.Validation("event trigger (%s) is not registered", t)
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
	Name          string
	Prefix        Prefix
	API           lambdaiface.LambdaAPI
	Features      map[string]Lambda
	BinaryName    string
	BinaryZipName string
}

func NewDefaultService(name string, prefix Prefix, rootDir string, api lambdaiface.LambdaAPI) *Service {
	layout := DefaultServiceLayout(rootDir)
	return NewService(name, prefix, layout, api)
}

func NewService(name string, prefix Prefix, layout ServiceLayout, api lambdaiface.LambdaAPI) *Service {
	return &Service{
		ServiceLayout: layout,
		Name:          name,
		Prefix:        prefix,
		API:           api,
		Features:      map[string]Lambda{},
	}
}

func (s *Service) LoadFromFilesystem() error {
	dirs, err := ioutil.ReadDir(s.LambdasDir())
	if err != nil {
		return failure.ToSystem(err, "ioutil.ReadDir failed")
	}

	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}

		lt, err := ToLambdaTrigger(d.Name())
		if err != nil {
			return failure.Wrap(err, "ToLambdaTrigger failed")
		}

		if err := s.LoadFromTriggerDir(lt); err != nil {
			return failure.Wrap(err, "s.LoadFromTriggerDir failed")
		}
	}

	return nil
}

func (s *Service) LoadFromTriggerDir(lt LambdaTrigger) error {
	if lt.Empty() {
		return failure.System("[lt] lambda trigger given is empty")
	}

	triggerDir := s.TriggerDir(lt)
	files, err := ioutil.ReadDir(triggerDir)
	if err != nil {
		return failure.ToSystem(err, "ioutil.ReadDir failed (%s)", lt)
	}

	for _, f := range files {
		if !f.IsDir() {
			continue
		}

		dir := filepath.Join(triggerDir, f.Name())
		files, err = ioutil.ReadDir(dir)
		if err != nil {
			return failure.ToSystem(err, "ioutil.ReadDir failed (%s), could not read lambda files", dir)
		}

		isMain := false
		for _, c := range files {
			// there must exist a main.go or else we can't deploy this lambda
			if c.Name() == DefaultLambdaGoFile {
				isMain = true
			}
		}

		if !isMain {
			continue
		}

		if err = s.AddFeature(lt, f.Name()); err != nil {
			return failure.Wrap(err, "s.AddFeature failed")
		}
	}

	return nil
}

func (s *Service) AddFeature(lt LambdaTrigger, name string) error {
	if lt.Empty() {
		return failure.System("[lt] LambdaTrigger is empty")
	}

	if name == "" {
		return failure.System("[name] lambda name is empty")
	}

	if s.Features == nil {
		s.Features = map[string]Lambda{}
	}

	rs := Lambda{
		Prefix:        s.Prefix,
		Service:       s.Name,
		Trigger:       lt,
		BaseName:      name,
		BinaryName:    s.BinaryName,
		BinaryZipName: s.BinaryZipName,
	}

	s.Features[name] = rs

	return nil
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
