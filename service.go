package sls

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"github.com/aws/aws-sdk-go/service/ssm/ssmiface"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/rsb/failure"

	"github.com/aws/aws-sdk-go/service/lambda/lambdaiface"
)

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

type LambdaAPI interface{}
type PStoreAPI interface{}

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
func (cl CodeLayout) TriggerDir(lt LambdaTrigger) string {
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

type MicroService struct {
	CodeLayout
	Resource  TFResource
	Name      ServiceName
	Account   AWSAccount
	Repo      Repo
	LambdaAPI lambdaiface.LambdaAPI
	PStoreAPI ssmiface.SSMAPI
	Features  map[string]Lambda
}

type MSConfig struct {
	RootDir   string
	Region    string
	Env       string
	RepoOwner string
	Repo      string
	App       string
	CLI       string
}

type FeatureDeployResult struct {
	Config *lambda.FunctionConfiguration
}

type FeatureDeployInput struct {
	Name        string
	ServiceName string
	CodeDir     *string
	BuildDir    *string
	BinName     *string
	ZipName     *string
	IsEnvOnly   bool
	Lambda      Lambda
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

func (s *MicroService) UpdateFeatureCode(ctx context.Context, in FeatureDeployInput) (*FeatureDeployResult, error) {
	var err error
	var compileResult *CompileResult

	if s.LambdaAPI == nil {
		return nil, failure.System("s.LambdaAPI is not initialized")
	}

	feature := in.Lambda
	if in.CodeDir == nil {
		compileResult, err = s.BuildFeature(feature)
	} else {
		compileResult, err = s.BuildFeature(in.Lambda, *in.CodeDir)
	}
	if err != nil {
		return nil, failure.Wrap(err, "s.BuildFeature failed")
	}

	zipName := DefaultBinaryZipName
	if in.ZipName != nil && *in.ZipName != "" {
		zipName = *in.ZipName
	}

	zipFile := filepath.Join(compileResult.BuildDir, zipName)
	if err := Zip(zipFile, compileResult.BinaryPath); err != nil {
		return nil, failure.Wrap(err, "Zip failed for (%s)", compileResult.BinaryPath)
	}

	zipData, err := ioutil.ReadFile(zipFile)
	if err != nil {
		return nil, failure.ToSystem(err, "ioutil.ReadDir failed")
	}

	funcName := aws.String(feature.QualifiedName)
	awsIn := lambda.UpdateFunctionCodeInput{
		FunctionName: funcName,
		ZipFile:      zipData,
	}

	out, err := s.LambdaAPI.UpdateFunctionCodeWithContext(ctx, &awsIn)
	if err != nil {
		return nil, failure.ToSystem(err, "s.LambdaAPI.UpdateFunctionCodeWithContext failed (%+v)", compileResult)
	}

	dr := FeatureDeployResult{
		Config: out,
	}

	return &dr, nil
}

func (s *MicroService) UpdateFeaturePStoreEnv(ctx context.Context, feature Lambda) (*FeatureDeployResult, error) {
	if s.PStoreAPI == nil {
		return nil, failure.System("s.PStore api is not initialized")
	}

	if feature.Conf == nil {
		return nil, failure.System("feature.Conf is not initialized")
	}

	envVars, err := feature.Conf.ProcessParamStore(s.PStoreAPI, s.Name.AppLabel(), false)
	if err != nil {
		return nil, failure.Wrap(err, "feature.Conf.ProcessParamStore failed (%s,%s)", s.Name, feature.Name)
	}
	feature.Env = envVars

	funcName := aws.String(feature.QualifiedName)

	dataIn := lambda.UpdateFunctionConfigurationInput{
		FunctionName: funcName,
		Environment:  feature.ToAWSEnv(),
	}

	out, err := s.LambdaAPI.UpdateFunctionConfigurationWithContext(ctx, &dataIn)
	if err != nil {
		return nil, failure.ToSystem(err, "api.UpdateFunctionConfigurationWithContext failed")
	}

	dr := FeatureDeployResult{
		Config: out,
	}

	return &dr, nil
}

func (s *MicroService) UpdateEnvVars(title string, config map[string]string) error {
	feature, err := s.Feature(title)
	if err != nil {
		return failure.Wrap(err, "s.Feature failed")
	}

	feature.Env = config

	s.Features[title] = feature
	return nil
}

func (s *MicroService) Invoke(ctx context.Context, feature Lambda, payload []byte) (InvokeResult, error) {
	var result InvokeResult
	if s.LambdaAPI == nil {
		return result, failure.System("s.LambdaAPI is not initialized")
	}

	in := NewDefaultInvokeInput(feature.QualifiedName, payload)

	out, err := s.LambdaAPI.InvokeWithContext(ctx, in)
	if err != nil {
		return result, failure.ToSystem(err, "s.LambdaAPI failed")
	}

	result, err = ToInvokeResult(out, feature)
	if err != nil {
		return result, failure.Wrap(err, "ToInvokeResult failed")
	}

	return result, nil
}

func (s *MicroService) InvokeAPIGW(ctx context.Context, in InvokeAPIGWInput) (InvokeResultAPIGW, error) {
	var resp events.APIGatewayProxyResponse
	var result InvokeResultAPIGW
	var out InvokeResult

	feature := in.Feature

	if feature.Trigger != APIGWTrigger {
		return result, failure.System("feature trigger (%s) is not (%s)", feature.Trigger, APIGWTrigger)
	}

	payload, err := json.Marshal(in.Request)
	if err != nil {
		return result, failure.ToSystem(err, "json.Marshal failed for APIGatewayProxyRequest")
	}

	out, err = s.Invoke(ctx, feature, payload)
	if err != nil {
		return result, failure.Wrap(err, "s.Invoke failed")
	}

	if err := json.Unmarshal(out.Payload, &resp); err != nil {
		return result, failure.ToSystem(err, "json.Unmarshal failed for (events.APIGatewayProxyResponse")
	}

	result = InvokeResultAPIGW{
		InvokeResult: out,
	}

	if resp.StatusCode >= http.StatusBadRequest {
		var funcErr APIGWFuncErrorResponse
		if err := json.Unmarshal([]byte(resp.Body), &funcErr); err != nil {
			return result, failure.ToSystem(err, "json.Unmarshal failed for (APIGWFuncErrorResponse)")
		}

		result.Message = funcErr.Message
		result.LambdaStatusCode = funcErr.Status
		result.RequestID = funcErr.ID

		return result, nil
	}

	result.LambdaStatusCode = resp.StatusCode

	if in.Unmarshaler == nil {
		if err = json.Unmarshal([]byte(resp.Body), in.Data); err != nil {
			return result, failure.ToSystem(err, "json.Unmarshal failed for (in.Data)")
		}

		return result, nil
	}

	if err = in.Unmarshaler(resp.Body); err != nil {
		return result, failure.ToSystem(err, "in.Unmarshaler failed")
	}

	return result, nil
}

func NewDefaultInvokeInput(name string, payload []byte) *lambda.InvokeInput {
	return &lambda.InvokeInput{
		FunctionName:   aws.String(name),
		Payload:        payload,
		InvocationType: aws.String(DefaultLambdaInvokeType),
		LogType:        aws.String(DefaultLambdaInvokeLogType),
	}
}

func ToInvokeResult(out *lambda.InvokeOutput, feature Lambda) (InvokeResult, error) {
	var result InvokeResult
	if out == nil {
		return result, failure.System("out lambda.InvokeOutput is empty")
	}

	var version string
	if out.ExecutedVersion != nil {
		version = *out.ExecutedVersion
	}

	var funcErr string
	if out.FunctionError != nil {
		funcErr = *out.FunctionError
	}

	logData := ""
	var logResult string
	if out.LogResult != nil {
		data, err := base64.URLEncoding.DecodeString(*out.LogResult)
		if err != nil {
			return result, failure.ToSystem(err, "base64.URLEncoding.DecodeString failed")
		}
		logData = string(data)
	}

	var status int64
	if out.StatusCode != nil {
		status = *out.StatusCode
	}

	result = InvokeResult{
		Name:            feature.Name,
		Trigger:         feature.Trigger,
		Payload:         out.Payload,
		ExecutedVersion: version,
		FunctionError:   funcErr,
		LogResult:       logResult,
		StatusCode:      status,
		LogData:         logData,
	}

	return result, nil
}

type InvokeAPIGWInput struct {
	Feature     Lambda
	Data        interface{}
	Request     events.APIGatewayProxyRequest
	Unmarshaler func(body string) error
}

type InvokeResultAPIGW struct {
	InvokeResult
	LambdaStatusCode int
	Message          string
	RequestID        string
}

type InvokeResult struct {
	Name            string
	Trigger         LambdaTrigger
	ExecutedVersion string
	FunctionError   string
	LogResult       string
	Payload         []byte
	StatusCode      int64
	LogData         string
}

type APIGWFuncErrorResponse struct {
	Message string `json:"message"`
	ID      string `json:"id"`
	Status  int    `json:"status"`
}

func NewMicroService(in MSConfig) (*MicroService, error) {
	if in.RootDir == "" {
		return nil, failure.Validation("in.RootDir for (%s) is empty", in.App)
	}

	if in.Region == "" {
		in.Region = DefaultRegion.String()
	}

	name, err := NewServiceName(in.Region, in.Env, in.App)
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
		Features:   map[string]Lambda{},
	}
	return &service, nil
}

func (s *MicroService) String() string {
	return s.Name.QualifiedName()
}

func (s *MicroService) Feature(title string) (Lambda, error) {
	var l Lambda
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

		et, err := ToLambdaTrigger(d.Name())
		if err != nil {
			return failure.Wrap(err, "invalid lambda trigger name, ToEventTrigger failed")
		}

		if err := s.AddByTrigger(et); err != nil {
			return failure.Wrap(err, "s.AddByTrigger failed")
		}
	}

	return nil
}

func (s *MicroService) AddByTrigger(et LambdaTrigger) error {
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

func (s *MicroService) AddFeature(et LambdaTrigger, title string) error {
	var rs Lambda
	if et.IsEmpty() {
		return failure.System("[et] event trigger is empty")
	}

	if title == "" {
		return failure.System("[title] feature title is empty")
	}

	if s.Features == nil {
		s.Features = map[string]Lambda{}
	}

	qualified := fmt.Sprintf("%s-%s_%s", s.Name.QualifiedName(), et, title)
	rs = Lambda{
		Name:          title,
		QualifiedName: qualified,
		Trigger:       et,
		BinaryName:    DefaultOutputName,
		BinaryZipName: DefaultBinaryZipName,
	}

	s.Features[title] = rs
	return nil
}

func (s *MicroService) BuildFeature(feature Lambda, codeDir ...string) (*CompileResult, error) {

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

func (s *MicroService) BuildFeatureCmd(in FeatureDeployInput) (*exec.Cmd, error) {
	targetDir := filepath.Join(s.LambdasDir(), in.Lambda.CodeDir())
	if in.CodeDir != nil {
		targetDir = *in.CodeDir
	}

	buildDir := s.BuildDir()
	if in.BuildDir != nil {
		buildDir = *in.BuildDir
	}

	binName := in.Lambda.BinaryName
	if in.BinName != nil {
		binName = *in.BinName
	}

	cmd, err := NewGoBuildCmd(buildDir, binName, targetDir)
	if err != nil {
		return nil, failure.Wrap(err, "NewGoBuildCmd failed")
	}

	return cmd, nil
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

func NewIAC(prefix Prefix, tfVersion, binDir, ResourceDir, repoOwner, repoRef string, isBranch bool) IAC {
	return IAC{
		BinaryDir:       binDir,
		BinaryName:      TerraformName,
		Version:         tfVersion,
		GlobalResources: NewGlobalResources(prefix, ResourceDir, repoOwner, repoRef, isBranch),
	}
}

func NewGlobalResources(prefix Prefix, rootDir, repoOwner, repoRef string, isBranch bool) GlobalResources {
	gd := filepath.Join(rootDir, "global")
	rd := filepath.Join(gd, RemoteStateTF)
	md := filepath.Join(gd, MessagingTF)
	kd := filepath.Join(gd, KeyPairTF)
	cd := filepath.Join(gd, CognitoTF)
	dd := filepath.Join(gd, LambdaDeployTF)
	nd := filepath.Join(gd, NetworkingTF)

	return GlobalResources{
		RemoteState: NewTFResource(rd, prefix, RemoteStateTF),
		RootDir:     rootDir,
		Repo:        NewGlobalResourceRepo(repoRef, repoOwner, isBranch),
		Config: map[string]TFResource{
			MessagingTF:    NewTFResource(md, prefix, MessagingTF),
			KeyPairTF:      NewTFResource(kd, prefix, KeyPairTF),
			CognitoTF:      NewTFResource(cd, prefix, CognitoTF),
			LambdaDeployTF: NewTFResource(dd, prefix, LambdaDeployTF),
			NetworkingTF:   NewTFResource(nd, prefix, NetworkingTF),
		},
	}
}

func NewGitHubConfig(owner string) VCS {
	return VCS{
		URL:   GithubURL,
		Owner: owner,
	}
}

func NewGlobalResourceRepo(owner, gitRefName string, isBranch bool) Repo {
	return Repo{
		Name:            LocalTerraformRepoName,
		Owner:           owner,
		IsBranch:        isBranch,
		RefName:         gitRefName,
		DefaultProtocol: SSHProtocol,
	}
}
func NewTFResource(rootDir string, prefix Prefix, label string) TFResource {
	vars := map[string]string{
		"env": prefix.Env(),
	}

	return TFResource{
		Dir:       rootDir,
		StateFile: "",
		PlanFile:  filepath.Join(rootDir, fmt.Sprintf("%s.plan.out", label)),
		Name:      label,
		Backend:   NewTFBackend(prefix, label),
		Vars:      vars,
	}
}

func NewTFBackend(prefix Prefix, label string) TFBackend {
	if label == RemoteStateTF {
		return TFBackend{}
	}

	return TFBackend{
		Bucket:      fmt.Sprintf("%s-tf-%s", prefix, RemoteStateTF),
		Key:         fmt.Sprintf("%s/%s.tfstate", label, label),
		Region:      prefix.Region,
		DynamoTable: fmt.Sprintf("%s-tf-%s-lock", prefix, RemoteStateTF),
	}
}
