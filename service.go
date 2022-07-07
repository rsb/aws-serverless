package sls

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
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

type Feature struct {
	Name          string
	QualifiedName string
	Trigger       InvokeTrigger
	BinaryName    string
	BinaryZipName string
	Conf          Configurable
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

// ServiceName is used for fender microservices. These are a repository of lambdas and as
// such the Microservice is not a physical aws resource but rather a collection of resources
// which include lambdas, apigw, sns queues, dynamodb etc... This name would act as the Prefix
// for those physical resources
type ServiceName struct {
	Prefix
	Title string
}

func NewServiceName(env string, title string) (ServiceName, error) {
	var name ServiceName

	prefix, err := DefaultPrefix(env)
	if err != nil {
		return name, failure.Wrap(err, "DefaultPrefix failed")
	}

	name = ServiceName{
		Prefix: prefix,
		Title:  title,
	}

	return name, nil
}

func (sn ServiceName) AppTitle() string {
	return sn.Title
}

func (sn ServiceName) QualifiedName() string {
	return fmt.Sprintf("%s-%s", sn.Prefix.String(), sn.AppTitle())
}

func (sn ServiceName) String() string {
	return sn.QualifiedName()
}

type Features map[string]Feature

type CodeLayout struct {
	Root      string
	Lambdas   string
	CLI       string
	Infra     string
	Build     string
	Terraform string
}

func (cl CodeLayout) RootDir() string {
	return cl.Root
}

func (cl CodeLayout) LambdasDir() string {
	return filepath.Join(cl.RootDir(), cl.Lambdas)
}

func (cl CodeLayout) InfraDir() string {
	return filepath.Join(cl.RootDir(), cl.Infra)
}

func (cl CodeLayout) TerraformDir() string {
	return filepath.Join(cl.InfraDir(), cl.Terraform)
}

func (cl CodeLayout) BuildDir() string {
	return cl.Build
}

func (cl CodeLayout) CLIDir() string {
	return filepath.Join(cl.RootDir(), cl.CLI)
}

func (cl CodeLayout) TriggerDir(et InvokeTrigger) string {
	return filepath.Join(cl.LambdasDir(), et.String())
}

func DefaultCodeLayout(root, cliPath string) CodeLayout {
	return CodeLayout{
		Root:      root,
		Lambdas:   DefaultLambdasDir,
		Infra:     DefaultInfraDir,
		Terraform: DefaultTerraform,
		Build:     DefaultBuildDir,
		CLI:       cliPath,
	}
}

/*
	Inputs for microservices

	1) root directory - absolute path to the microservice codebase
	2) repo 					- GitHub repository information used to checkout the code base
	3) app title      - the base name in our AWS resource naming convention for microservices
	4) cli title 			- the name of microservice's cli binary used to manage this cli
	5) env 						- name of the aws environment the microservice will run in
	6) region 				- default aws region when managing aws resources through the sdk
	7) profile 				- the aws profile used by system managing the environment. used for creds

*/
type MicroService struct {
	CodeLayout
	Resource TFResource
	Account  AWSAccount
	Name     ServiceName
	Repo     Repo
	Features map[string]Feature
}

type MicroServiceIn struct {
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

type FeatureDeployment struct {
	Name        string
	ServiceName string
	CodeDir     *string
	BuildDir    *string
	BinName     *string
	ZipName     *string
	IsEnvOnly   bool
	Lambda      Feature
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
func (s *MicroService) BuildFeatureCode(buildDir, binaryName, sourceDir string) (*CompileResult, error) {
	cmd, err := NewGoBuildCmd(buildDir, binaryName, sourceDir)
	if err != nil {
		return nil, failure.Wrap(err, "NewGoBuildCmd failed for (%s,%s,%s)", buildDir, binaryName, sourceDir)
	}

	if err := cmd.Run(); err != nil {
		return nil, failure.Wrap(err, "could not build (%s) cmd.Run failed.", binaryName)
	}

	rs := CompileResult{
		BuildDir:   buildDir,
		BinaryName: binaryName,
		BinaryPath: filepath.Join(buildDir, binaryName),
		CodeDir:    sourceDir,
	}

	return &rs, nil
}

func NewMicroService(in MicroServiceIn) (*MicroService, error) {
	if in.RootDir == "" {
		return nil, failure.Config("in.RootDir for (%s) is empty", in.App)
	}

	name, err := NewServiceName(in.Env, in.App)
	if err != nil {
		return nil, failure.Wrap(err, "NewServiceName failure")
	}

	repo := NewRepo(in.RepoOwner, in.Repo, DefaultRepoRefName, true)

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

func (s *MicroService) Feature(title string) (Feature, error) {
	var l Feature
	f, ok := s.Features[title]
	if !ok {
		return l, failure.NotFound("feature (%s)", title)
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

func (s *MicroService) AddByTrigger(et InvokeTrigger) error {
	if et.IsEmpty() {
		return failure.System("[et] event trigger is empty")
	}

	triggerDir := s.TriggerDir(et)
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

		if err := s.AddFeature(et, f.Name()); err != nil {
			return failure.Wrap(err, "s.AddFeature failed")
		}
	}

	return nil
}

func (s *MicroService) AddFeature(et InvokeTrigger, title string) error {
	var rs Feature
	if et.IsEmpty() {
		return failure.System("[et] event trigger is empty")
	}

	if title == "" {
		return failure.System("[title] feature title is empty")
	}

	if s.Features == nil {
		s.Features = map[string]Feature{}
	}

	qualified := fmt.Sprintf("%s-%s_%s", s.Name.QualifiedName(), et, title)
	rs = Feature{
		Name:          title,
		QualifiedName: qualified,
		Trigger:       et,
		BinaryName:    DefaultOutputName,
		BinaryZipName: DefaultBinaryZipName,
	}

	s.Features[title] = rs
	return nil
}

type CompileResult struct {
	BuildDir   string
	BinaryName string
	BinaryPath string
	CodeDir    string
}

func (s *MicroService) BuildFeature(feature Feature, codeDir ...string) (*CompileResult, error) {

	targetDir := filepath.Join(s.LambdasDir(), feature.CodeDir())
	if len(codeDir) > 0 && codeDir[0] != "" {
		targetDir = codeDir[0]
	}

	binName := feature.BinaryName
	if binName == "" {
		binName = DefaultOutputName
	}
	rs, err := s.BuildFeatureCode(s.BuildDir(), binName, targetDir)
	if err != nil {
		return nil, failure.Wrap(err, "s.BuildFeatureCode failed")
	}

	return rs, nil
}
func (s *MicroService) GoPath() (string, error) {
	goExec, err := exec.LookPath(GoBinaryName)
	if err != nil {
		return "", failure.ToSystem(err, "exec.LookPath failed")
	}

	return goExec, nil
}

func (s *MicroService) UpdateEnvWithPStoreCmd(feature string) (*exec.Cmd, error) {
	rootDir := s.RootDir()

	goExec, err := s.GoPath()
	if err != nil {
		return nil, failure.ToSystem(err, "s.GoPath failed")
	}

	if feature == "" {
		return nil, failure.System("feature is empty")
	}

	mainGo := path.Join(s.CLIDir(), "main.go")

	cmd := exec.Cmd{
		Env:  os.Environ(),
		Dir:  rootDir,
		Path: goExec,
		Args: []string{
			goExec,
			"run",
			mainGo,
			"infra",
			"deploy",
			feature,
			"--env-only",
		},
	}
	return &cmd, nil
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
