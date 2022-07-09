package infra

import (
	"context"

	"github.com/rsb/failure"
	"github.com/rsb/sls"
	"github.com/rsb/sls/lambda"
	"github.com/spf13/cobra"
)

func SetupDeployCmd(in *Infra) error {
	if err := in.Validate(); err != nil {
		return failure.Wrap(err, "in.Validate failed")
	}

	if in.DeployCmd == nil {
		in.DeployCmd = DeployCmd
	}

	// We always want the RunE to use this function
	in.DeployCmd.RunE = in.RunDeploy

	in.ParentCmd.AddCommand(in.DeployCmd)
	var db DeployBind
	if err := Bind(in.DeployCmd, in.Viper, &db); err != nil {
		return failure.Wrap(err, "Bind failed for in.Deploy.Cmd")
	}

	return nil
}

var DeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "deploy lambdas to aws",
	Args:  cobra.MinimumNArgs(1),
}

type DeployBind struct {
	IsEnvOnly bool `conf:"cli:env-only, cli-u: Only update environment variables"`
}

type DeployConfig struct {
	DeployBind
	CmdConfig
}

func (i *Infra) RunDeploy(cmd *cobra.Command, args []string) error {
	if err := i.Validate(); err != nil {
		return failure.Wrap(err, "i.Validate failed")
	}

	var config DeployConfig

	if err := i.Process(cmd, &config); err != nil {
		return failure.Wrap(err, "i.Configure failed")
	}

	service, feature, err := i.LoadFeature(config.CmdConfig, args[0])
	if err != nil {
		return failure.Wrap(err, "i.LoadFeatureFromFirstArg")
	}
	ctx := context.Background()

	if config.IsEnvOnly {
		if err = i.DeployFeatureConfig(ctx, service.Name.AppTitle(), feature, config); err != nil {
			return failure.Wrap(err, "i.DeployFeatureConfig failed")
		}
		return nil
	}

	settings := service.NewBuildSettings(feature)
	if err = i.DeployFeatureCode(ctx, feature, settings, config); err != nil {
		return failure.Wrap(err, "i.DeployFeature failed")
	}

	return nil
}

func (i *Infra) DeployFeatureConfig(ctx context.Context, appTitle string, feature sls.Feature, config DeployConfig) error {
	vars, err := i.FeatureParams(ctx, appTitle, feature)
	if err != nil {
		return failure.Wrap(err, "i.FeatureParams failed")
	}

	vars = i.StripAppTitle(appTitle, vars)
	settings := lambda.FeatureSettings{
		QualifiedName: feature.QualifiedName,
		EnvVars:       vars,
	}

	report, err := i.LambdaAPI.UpdateConfig(ctx, settings)
	if err != nil {
		return failure.Wrap(err, "i.LambdaAPI.UpdateConfig failed")
	}

	if config.CmdConfig.Verbose {
		i.DisplayJson(report)
	}

	return nil
}

func (i *Infra) DeployFeatureCode(ctx context.Context, feature sls.Feature, settings sls.BuildSettings, config DeployConfig) error {
	result, err := i.LambdaAPI.Compile(settings)
	if err != nil {
		return failure.Wrap(err, "i.LambdaAPI.Compile failed")
	}

	in := lambda.CodePayload{
		QualifiedName: feature.QualifiedName,
		ZipFile:       result.ZipData,
	}
	report, err := i.LambdaAPI.UpdateCode(ctx, in)
	if err != nil {
		return failure.Wrap(err, "i.LambdaAPI.UpdateCode failed")
	}

	if config.CmdConfig.Verbose {
		i.DisplayJson(report)
	}

	return nil
}
