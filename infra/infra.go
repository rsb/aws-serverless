// Package infra is responsible for infrastructure related concerns of the app
// like deploying and configuration
package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mitchellh/go-homedir"

	"github.com/rsb/conf"
	"github.com/rsb/failure"

	"github.com/spf13/viper"

	"github.com/rsb/sls"
	"github.com/rsb/sls/lambda"
	"github.com/spf13/cobra"
)

type ParamStorage interface {
	Param(ctx context.Context, key string) (string, error)
	Path(ctx context.Context, path string, recursive ...bool) (map[string]string, error)
	Collect(ctx context.Context, key ...string) (map[string]string, []string, error)
	Delete(ctx context.Context, key string) (string, error)
	Put(ctx context.Context, key, value string, overwrite ...bool) (string, error)
	EnsurePathPrefix(path string) string
}

type LambdaDeployments interface {
	Compile(data sls.BuildSettings) (sls.BuildResult, error)
	UpdateCode(ctx context.Context, payload lambda.CodePayload) (*lambda.FeatureUpdateReport, error)
	UpdateConfig(ctx context.Context, in lambda.FeatureSettings) (*lambda.FeatureUpdateReport, error)
}

type EnvIdentity interface {
	EnvName() string
}

type CmdConfig struct {
	Verbose         bool   `conf:"          global-flag,                      cli:verbose,               cli-s:v,   cli-u:Show verbose output"`
	IsAll           bool   `conf:"          global-flag,                      cli:all,            cli-u: apply to the whole service"`
	SkipDefaults    bool   `conf:"          global-flag,                      cli:skip-defaults,  cli-u: skip default values"`
	WithTrigger     bool   `conf:"          global-flag, env:-,               cli:name-includes-trigger, cli-s:t,   cli-u:For when feature name is given with the trigger (ex apigw_feature)"`
	IsText          bool   `conf:"          global-flag, env:CLI_FORMAT_TEXT, cli:text,           cli-u:Use plain text instead of json'"`
	IsQualifiedName bool   `conf:"          global-flag, env:QUALIFIED_NAMES, cli:qualified-name, cli-u:Display names as fully qualified"`
	Env             string `conf:"required, global-flag, env:ENV,             cli:env, cli-s:e,   cli-u:Application env"`
}

func (c CmdConfig) EnvName() string {
	return c.Env
}

func (c CmdConfig) NameIncludesTrigger() bool {
	return c.WithTrigger
}

func (c CmdConfig) IsFullyQualifiedName() bool {
	return c.IsQualifiedName
}

type Infra struct {
	Stdout             io.ReadWriteCloser
	Stderr             io.ReadWriteCloser
	Viper              *viper.Viper
	PStoreAPI          ParamStorage
	LambdaAPI          LambdaDeployments
	Service            *sls.MicroService
	ServiceConstructor func(config EnvIdentity) (*sls.MicroService, error)
	ParentCmd          *cobra.Command
	Prefix             []string

	DeployCmd       *cobra.Command
	EnvCmd          *cobra.Command
	EnvExportCmd    *cobra.Command
	PStoreCmd       *cobra.Command
	PStoreImportCmd *cobra.Command
	PStoreDeleteCmd *cobra.Command
	PStoreExportCmd *cobra.Command
	InvokeCmd       *cobra.Command
}

func SetupCommands(i *Infra) error {
	if err := SetupInfraCmd(i); err != nil {
		return failure.Wrap(err, "SetupInfraCmd failed")
	}

	if err := SetupEnvCmd(i); err != nil {
		return failure.Wrap(err, "SetupEnvCmd failed")
	}

	if err := SetupDeployCmd(i); err != nil {
		return failure.Wrap(err, "SetupDeployCmd failed")
	}

	if err := SetupParamStoreCmd(i); err != nil {
		return failure.Wrap(err, "SetupParamStoreCmd failed")
	}

	return nil
}

func (i *Infra) Validate() error {
	if i == nil {
		return failure.InvalidState("Infra is nil")
	}

	if i.ParentCmd == nil {
		return failure.InvalidState("i.ParentCmd is nil. Parent cobra cmd is required")
	}

	if i.Viper == nil {
		return failure.InvalidState("i.Viper is nil. Instance of viper is required")
	}

	if i.ServiceConstructor == nil {
		return failure.InvalidState("i.ServiceConstructor is nil. ServiceConstructor is require for all cmds")
	}

	return nil
}

func SetupInfraCmd(i *Infra) error {
	if err := i.Validate(); err != nil {
		return failure.Wrap(err, "i.Validate failed")
	}

	var config CmdConfig
	if err := Bind(i.ParentCmd, i.Viper, &config); err != nil {
		return failure.Wrap(err, "Bind failed for in.ParentCmd")
	}

	if i.Stdout == nil {
		i.Stdout = os.Stdout
	}

	if i.Stderr == nil {
		i.Stderr = os.Stderr
	}
	return nil
}

func (i *Infra) LoadService(config CmdConfig) (*sls.MicroService, error) {
	var service *sls.MicroService

	service, err := i.ServiceConstructor(config)
	if err != nil {
		return service, failure.Wrap(err, "i.ServiceConstructor failed")
	}

	return service, nil
}

func (i *Infra) LoadFeature(config CmdConfig, name string) (*sls.MicroService, sls.Feature, error) {
	var feature sls.Feature

	service, err := i.LoadService(config)
	if err != nil {
		return nil, feature, failure.Wrap(err, "i.LoadService failed")
	}

	// It is important to note that if a feature is not found and the isTrigger flag is true
	// then we will assume the name is in the form of <trigger>_<name>. So we will split it out
	// and search again. This is why we are return when the error is nil.
	feature, err = service.Feature(name)
	// NOTE: Success case, not error case
	if err == nil {
		return service, feature, nil
	}

	wrappedErr := failure.Wrap(err, "service.Feature failed for (%s)", name)
	if !failure.IsNotFound(err) {
		return service, feature, wrappedErr
	}

	// Since the trigger was not configured to be used then return the error
	if !config.NameIncludesTrigger() {
		return service, feature, wrappedErr
	}

	parts := strings.Split(name, "_")
	if len(parts) == 1 {
		return service, feature, failure.Wrap(err, "invalid format, should be (<eventTrigger>_<featureName>)")
	}

	// We only care that the trigger is valid, we don't need the type.
	_, err = sls.InvokeTriggerFromString(parts[0])
	if err != nil {
		return service, feature, failure.Wrap(err, "sls.ToEventTrigger failed")
	}

	// Now that we verified the first element was a valid event trigger the
	// rest of the elements make up the feature name
	fName := strings.Join(parts[1:], "_")
	feature, err = service.Feature(fName)
	if err != nil {
		return service, feature, failure.Wrap(err, "service.Feature failed, even when taking into account the trigger (%s, %s)", parts[0], fName)
	}

	return service, feature, nil
}

func (i *Infra) Process(cmd *cobra.Command, c interface{}) error {
	if err := Process(cmd, i.Viper, c, i.Prefix...); err != nil {
		return failure.Wrap(err, "Process failed for (DeployConfig)")
	}

	return nil
}

func Process(cmd *cobra.Command, v *viper.Viper, spec interface{}, prefix ...string) error {
	if err := conf.ProcessCLI(cmd, v, spec, prefix...); err != nil {
		return failure.Wrap(err, "goKitConf.ProcessCLI failed")
	}

	return nil
}

func Bind(c *cobra.Command, v *viper.Viper, config interface{}) error {
	if err := conf.BindCLI(c, v, config); err != nil {
		return failure.Wrap(err, "goKitConf.BindCLI failed")
	}

	return nil
}

func (i *Infra) CheckFailure(err error) {
	if err == nil {
		return
	}

	if _, fErr := fmt.Fprintf(i.Stderr, "[infra] %+v\n", err); fErr != nil {
		fmt.Println("[infra:CheckFailure]:fmt.Fprintf failed:", fErr.Error(), ":", err.Error())
	}
	os.Exit(1)
}

func (i *Infra) WriteJson(p string, d interface{}) {
	data, err := json.Marshal(d)
	if err != nil {
		i.CheckFailure(failure.ToSystem(err, "json.Marshal failed in WriteJson"))
	}

	fp := Filepath{}
	if err = fp.Decode(p); err != nil {
		i.CheckFailure(failure.ToSystem(err, "fp.Decode failed (%s) in WriteJson", p))
	}

	file, err := os.Create(fp.Path)
	if err != nil {
		i.CheckFailure(failure.ToSystem(err, "os.Create failed (%s) in WriteJson", p))
	}
	defer func() { _ = file.Close() }()

	if _, err = file.Write(data); err != nil {
		i.CheckFailure(failure.ToSystem(err, "file.Write failed (%s) in WriteJSON", p))
	}
}

func (i *Infra) DisplayJson(d interface{}) {
	data, err := json.Marshal(d)
	i.CheckFailure(err)

	i.Display(string(data))
	return
}

func (i *Infra) DisplayErrorJson(d interface{}) {
	data, err := json.Marshal(d)
	i.CheckFailure(err)

	i.DisplayError(string(data))
	return
}

func (i *Infra) DisplayError(data string) {
	_, err := fmt.Fprintf(i.Stderr, data)
	i.CheckFailure(err)

	return
}

func (i *Infra) Display(data string) {
	_, err := fmt.Fprintf(i.Stdout, data)
	i.CheckFailure(err)

	return
}

// Filepath is used as a custom decoder which will take a configuration string
// and resolve a ~ to the absolute path of the home directory. If ~ is not
// present it treated as a normal path to a directory
type Filepath struct {
	Path string
}

func (d *Filepath) String() string {
	return d.Path
}

func (d *Filepath) IsEmpty() bool {
	return d.Path == ""
}

func (d *Filepath) Decode(v string) error {
	if v == "" {
		return nil
	}

	path, err := homedir.Expand(v)
	if err != nil {
		return failure.ToSystem(err, "homedir.Expand failed")
	}

	d.Path = path
	return nil
}
