// Package sls exposes behavior and types that are used to manage day to day
// development, deployment, inspecting and testing of serverless applications
// running is AWS
package sls

import (
	"fmt"

	"github.com/spf13/viper"

	"github.com/rsb/failure"
)

const (
	amznTraceIDCtxKey          = "x-amzn-trace-id"
	DefaultOutputName          = "bootstrap"
	DefaultBinaryZipName       = "deployment.zip"
	DefaultInfraDir            = "infra/local"
	DefaultTerraform           = "terraform"
	DefaultLambdasDir          = "app/lambdas"
	DefaultBuildDir            = "/tmp"
	DefaultRepoRefName         = "main"
	DefaultLambdaInvokeType    = "RequestResponse"
	DefaultLambdaInvokeLogType = "Tail"
	DefaultLambdaGoFile        = "main.go"
	DefaultAppDirName          = "app"
	DefaultLambdaDirName       = "lambdas"
	DefaultInfraDirName        = "infra"
	DefaultBuildDirName        = "build"
	DefaultTerraformDirName    = "terraform"

	LocalTerraformRepoName = "local-terraform"
	GithubURL              = "github.com"
	TerraformName          = "terraform"
	SSHProtocol            = Protocol("ssh")
	HTTPSProtocol          = Protocol("https")
	CLIProtocol            = Protocol("cmds")
	CLITemplate            = "%s/%s"
	SSHTemplate            = "git@%s:%s/%s.git"
	HTTPSTemplate          = "https://%s/%s/%s.git"

	RemoteStateTF  = "remote-state"
	LambdaDeployTF = "lambda-deploy-bucket"
	KeyPairTF      = "key-pair"
	MessagingTF    = "messaging"
	CognitoTF      = "cognito"
	NetworkingTF   = "networking"
)

// Prefix represents our naming prefix, used when creating resources with
// terraform. Every fender resource has a prefix to encode information about
// it. The fender prefix is laid out as follows:
// <region_code>-<env>
// region_code - this is the aws region code so `use1 [us-east-1]`
// env 				 - the environment this resource is provisioned in. `[prod,qa,active,<developer_code>]`
type Prefix struct {
	Region  Region
	EnvName string
}

func NewPrefix(reg, env string) (Prefix, error) {
	var p Prefix
	if reg == "" {
		return p, failure.System("[reg] aws region is empty should be in the form of (us-east-1)")
	}

	region, err := ToRegion(reg)
	if err != nil {
		return p, failure.Wrap(err, "[reg] ToRegion failed")
	}

	if env == "" {
		return p, failure.System("[env] application environment is empty")
	}

	return Prefix{Region: region, EnvName: env}, nil
}

func DefaultPrefix(env string) (Prefix, error) {
	prefix, err := NewPrefix(DefaultRegion.String(), env)
	if err != nil {
		return prefix, failure.Wrap(err, "NewPrefix failed")
	}

	return prefix, nil
}

func (p Prefix) String() string {
	return fmt.Sprintf("%s-%s", p.Region.Code(), p.Env())
}

func (p Prefix) Env() string {
	return p.EnvName
}
func (p Prefix) RegionCode() string {
	return p.RegionCode()
}

func (p Prefix) AWSRegion() string {
	return p.Region.String()
}

func (p Prefix) IsValid() bool {
	return !p.Region.IsEmpty() && p.Env() != ""
}

// AWSAccount hold the profile used and the default region. We can use
// this to set ENV vars before running our code.
type AWSAccount struct {
	Region  Region
	Profile string
}

// VCS hold info about our version control system. In our case this is git/GitHub
type VCS struct {
	URL   string
	Owner string
}

type Protocol string

func (p Protocol) String() string {
	return string(p)
}

func (p Protocol) IsEmpty() bool {
	return p.String() == ""
}

type Repo struct {
	Name            string
	Owner           string
	IsBranch        bool
	RefName         string
	DefaultProtocol Protocol
}

func NewRepo(owner, name, refName string, isBranch bool) Repo {
	return Repo{
		Owner:           owner,
		Name:            name,
		RefName:         refName,
		IsBranch:        isBranch,
		DefaultProtocol: SSHProtocol,
	}
}

func (r Repo) URI(p ...Protocol) string {
	var protocol Protocol
	var uri string

	if len(protocol) > 0 {
		protocol = p[0]
	}

	if protocol.IsEmpty() {
		protocol = r.DefaultProtocol
	}

	switch protocol {
	case HTTPSProtocol:
		uri = fmt.Sprintf(HTTPSTemplate, GithubURL, r.Owner, r.Name)
	case CLIProtocol:
		uri = fmt.Sprintf(CLITemplate, r.Owner, r.Name)
	default:
		uri = fmt.Sprintf(SSHTemplate, GithubURL, r.Owner, r.Name)
	}

	return uri
}

type Configurable interface {
	ProcessEnv() error
	ProcessCLI(v *viper.Viper) error
	CollectParamsFromEnv(appTitle string) (map[string]string, error)
	ParamNames(appTitle string) ([]string, error)
	EnvNames() ([]string, error)
	EnvToMap() (map[string]string, error)
	SetPrefix(prefix string)
	GetPrefix()
	IsPrefixEnabled() bool
	MarkDefaultsAsExcluded()
	MarkDefaultsAsIncluded()
	SetExcludeDefaults(value bool)
	IsDefaultsExcluded() bool
}

type KeyPair struct {
	Name      string
	PublicKey string
}
