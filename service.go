package sls

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/aws/aws-lambda-go/events"

	"github.com/rsb/failure"
)

const (
	APIGWProxyTrigger      = InvokeTrigger("apigw")
	APIGWCustomAuthTrigger = InvokeTrigger("apigw-auth")
	AppSyncTrigger         = InvokeTrigger("appsync")
	CloudWatchEventTrigger = InvokeTrigger("cw-event")
	CloudWatchLogsTrigger  = InvokeTrigger("cw-log")
	CognitoTrigger         = InvokeTrigger("cognito")
	DDBTrigger             = InvokeTrigger("ddb")
	DDBStreamTrigger       = InvokeTrigger("ddb-stream")
	DirectTrigger          = InvokeTrigger("direct")
	KinesisStreamTrigger   = InvokeTrigger("kinesis-stream")
	SNSTrigger             = InvokeTrigger("sns")
	SQSTrigger             = InvokeTrigger("sqs")
	S3Trigger              = InvokeTrigger("s3")
	StepTrigger            = InvokeTrigger("sfn")
)

var (
	APIGWProxyEvent      = reflect.TypeOf(events.APIGatewayProxyRequest{})
	APIGWCustomAuthEvent = reflect.TypeOf(events.APIGatewayCustomAuthorizerRequest{})
	CloudWatchEvent      = reflect.TypeOf(events.CloudWatchEvent{})
	CloudWatchLogsEvent  = reflect.TypeOf(events.CloudwatchLogsEvent{})
	DDBEvent             = reflect.TypeOf(events.DynamoDBEvent{})
	DirectEvent          = reflect.TypeOf([]byte{})
	SNSEvent             = reflect.TypeOf(events.SNSEvent{})
	SQSEvent             = reflect.TypeOf(events.SQSEvent{})
	S3Event              = reflect.TypeOf(events.S3Event{})
)

type InvokeTrigger string

func (lt InvokeTrigger) String() string {
	return string(lt)
}

func (lt InvokeTrigger) IsEmpty() bool {
	return lt.String() == ""
}

func InvokeTriggerFromString(s string) (InvokeTrigger, error) {
	var t InvokeTrigger
	var err error
	switch strings.ToLower(s) {
	case APIGWProxyTrigger.String():
		t = APIGWProxyTrigger
	case DDBTrigger.String():
		t = DDBTrigger
	case DDBStreamTrigger.String():
		t = DDBStreamTrigger
	case DirectTrigger.String():
		t = DirectTrigger
	case CognitoTrigger.String():
		t = CognitoTrigger
	case S3Trigger.String():
		t = S3Trigger
	case StepTrigger.String():
		t = StepTrigger
	case SNSTrigger.String():
		t = SNSTrigger
	default:
		err = failure.System("event trigger (%s) is not mapped", t)
	}

	return t, err
}

func InvokeTriggerFromEvent(t reflect.Type) (InvokeTrigger, error) {
	var it InvokeTrigger
	var err error

	switch t {
	case APIGWProxyEvent:
		it = APIGWProxyTrigger
	case APIGWCustomAuthEvent:
		it = APIGWCustomAuthTrigger
	case CloudWatchEvent:
		it = CloudWatchEventTrigger
	case CloudWatchLogsEvent:
		it = CloudWatchLogsTrigger
	case DDBEvent:
		it = DDBTrigger
	case DirectEvent:
		it = DirectTrigger
	case SNSEvent:
		it = SNSTrigger
	case SQSEvent:
		it = SQSTrigger
	case S3Event:
		it = S3Trigger
	}

	return it, err
}

type ConcreteHandlerFn func() (out interface{}, err error)

// ServiceName is used for microservices. These are a repository of lambdas and as
// such the Microservice is not a physical aws resource but rather a collection of resources
// which include lambdas, apigw, sns queues, dynamodb etc... This name would act as the Prefix
// for those physical resources
type ServiceName struct {
	Prefix
	Label string
}

func NewServiceName(region, env string, label string) (ServiceName, error) {
	var name ServiceName

	prefix, err := NewPrefix(region, env)
	if err != nil {
		return name, failure.Wrap(err, "DefaultPrefix failed")
	}

	name = ServiceName{
		Prefix: prefix,
		Label:  label,
	}

	return name, nil
}

func (sn ServiceName) AppLabel() string {
	return sn.Label
}

func (sn ServiceName) QualifiedName() string {
	return fmt.Sprintf("%s-%s", sn.Prefix.String(), sn.AppLabel())
}

func (sn ServiceName) String() string {
	return sn.QualifiedName()
}

// CodeLayout is a collection of directories which layout where the
// required code is located in order to build and deploy aws lambdas.
//
// This layout makes the following assumptions:
// 1) App is always under the Root directory
// 2) Lambdas are always under the App directory
// 3) Infra is always under the Root directory
// 4) Build is always under the Infra directory
// 5) Terraform is always under the Infra directory
type CodeLayout struct {
	Root      string
	App       string
	Lambdas   string
	CLI       string
	Infra     string
	Build     string
	Terraform string
}

func DefaultCodeLayout(dir, cliPath string) CodeLayout {
	return CodeLayout{
		Root:      dir,
		CLI:       cliPath,
		App:       DefaultAppDirName,
		Lambdas:   DefaultLambdaDirName,
		Infra:     DefaultInfraDirName,
		Build:     DefaultBuildDirName,
		Terraform: DefaultTerraformDirName,
	}
}

func (cl CodeLayout) RootDir() string {
	return cl.Root
}

func (cl CodeLayout) AppDir() string {
	return filepath.Join(cl.RootDir(), cl.App)
}

func (cl CodeLayout) LambdasDir() string {
	return filepath.Join(cl.AppDir(), cl.Lambdas)
}
func (cl CodeLayout) TriggerDir(lt InvokeTrigger) string {
	return filepath.Join(cl.LambdasDir(), lt.String())
}

func (cl CodeLayout) InfraDir() string {
	return filepath.Join(cl.Root, cl.Infra)
}

func (cl CodeLayout) BuildDir() string {
	return filepath.Join(cl.InfraDir(), cl.Build)
}

func (cl CodeLayout) TerraformDir() string {
	return filepath.Join(cl.InfraDir(), cl.Terraform)
}

func (cl CodeLayout) CLIDir() string {
	return filepath.Join(cl.RootDir(), cl.CLI)
}

type Feature struct {
	Name          string
	QualifiedName string
	Trigger       InvokeTrigger
	BinaryName    string
	BinaryZipName string
	Conf          ConfigurableFeature
	Env           map[string]string
}

func (l Feature) AddEnv(name, value string) {
	l.Env[name] = value
}

func (l Feature) TriggerDir() string {
	return l.Trigger.String()
}

func (l Feature) CodeDir() string {
	return filepath.Join(l.TriggerDir(), l.Name)
}

func (l Feature) NameWithTrigger() string {
	return fmt.Sprintf("%s_%s", l.Trigger, l.Name)
}

func (l Feature) String() string {
	return l.Name
}

type MicroService struct {
	CodeLayout
	Resource TFResource
	Name     ServiceName
	Account  AWSAccount
	Repo     Repo
	Features map[string]Feature
}

type MSSettings struct {
	RootDir      string
	Region       string
	Env          string
	RepoOwner    string
	Repo         string
	RepoRef      string
	IsRepoBranch bool
	App          string
	CLI          string
}

type BuildSettings struct {
	CodeDir     string
	BuildDir    string
	BinName     string
	BinPath     string
	SkipZipping bool
	ZipName     string
}

type BuildResult struct {
	Settings BuildSettings
	ZipName  string
	ZipData  []byte
}

func NewMicroService(in MSSettings) (*MicroService, error) {
	if in.RootDir == "" {
		return nil, failure.Validation("in.RootDir for (%s) is empty", in.App)
	}

	name, err := NewServiceName(in.Region, in.Env, in.App)
	if err != nil {
		return nil, failure.Wrap(err, "NewServiceName failure")
	}

	repo := NewRepo(in.RepoOwner, in.Repo, in.RepoRef, in.IsRepoBranch)

	cliPath := fmt.Sprintf("app/cli/%s", in.CLI)
	layout := DefaultCodeLayout(in.RootDir, cliPath)

	rs := NewTFResource(layout.TerraformDir(), name.Prefix, in.Repo)
	service := MicroService{
		CodeLayout: layout,
		Resource:   rs,
		Name:       name,
		Repo:       repo,
		Features:   map[string]Feature{},
	}

	return &service, nil
}

func (s *MicroService) String() string {
	return s.Name.QualifiedName()
}

func (s *MicroService) NewBuildSettings(feature Feature) BuildSettings {
	binName := feature.BinaryName
	buildDir := s.BuildDir()
	return BuildSettings{
		CodeDir:  filepath.Join(s.LambdasDir(), feature.CodeDir()),
		BuildDir: s.BuildDir(),
		BinName:  binName,
		ZipName:  feature.BinaryZipName,
		BinPath:  filepath.Join(buildDir, binName),
	}
}

func (s *MicroService) Feature(title string) (Feature, error) {
	var f Feature
	f, ok := s.Features[title]
	if !ok {
		return f, failure.NotFound("feature (%s)", title)
	}

	return f, nil
}

func (s *MicroService) LoadFeaturesFromFilesystem() error {
	dir := s.LambdasDir()
	dirs, err := ioutil.ReadDir(dir)
	if err != nil {
		return failure.ToSystem(err, "ioutil.ReadDir failed")
	}

	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}

		et, err := InvokeTriggerFromString(d.Name())
		if err != nil {
			return failure.Wrap(err, "invalid lambda trigger name, ToEventTrigger failed")
		}

		if err := s.AddByTrigger(et); err != nil {
			return failure.Wrap(err, "s.AddByTrigger failed")
		}
	}

	return nil
}

func (s *MicroService) AddByTrigger(lt InvokeTrigger) error {
	if lt.IsEmpty() {
		return failure.System("[lt] event trigger is empty")
	}

	triggerDir := s.TriggerDir(lt)
	files, err := ioutil.ReadDir(triggerDir)
	if err != nil {
		return failure.ToSystem(err, "ioutil.ReadDir failed")
	}

	for _, f := range files {
		if !f.IsDir() {
			continue
		}

		codeDir := filepath.Join(triggerDir, f.Name())
		codeFiles, err := ioutil.ReadDir(codeDir)
		if err != nil {
			return failure.ToSystem(err, "ioutil.ReadDir failed")
		}

		foundMain := false
		for _, c := range codeFiles {
			if c.Name() == "main.go" {
				foundMain = true
			}
		}

		if !foundMain {
			continue
		}

		if err := s.AddFeature(lt, f.Name()); err != nil {
			return failure.Wrap(err, "s.AddFeature failed")
		}
	}

	return nil
}

func (s *MicroService) AddFeature(lt InvokeTrigger, title string) error {
	var rs Feature
	if lt.IsEmpty() {
		return failure.System("[lt] event trigger is empty")
	}

	if title == "" {
		return failure.System("[title] feature title is empty")
	}

	if s.Features == nil {
		s.Features = map[string]Feature{}
	}

	qualified := fmt.Sprintf("%s-%s_%s", s.Name.QualifiedName(), lt, title)
	rs = Feature{
		Name:          title,
		QualifiedName: qualified,
		Trigger:       lt,
		BinaryName:    DefaultOutputName,
		BinaryZipName: DefaultBinaryZipName,
	}

	s.Features[title] = rs
	return nil
}
