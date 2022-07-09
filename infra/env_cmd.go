package infra

import (
	"encoding/json"
	"fmt"

	"github.com/rsb/failure"
	"github.com/rsb/sls"
	"github.com/spf13/cobra"
)

func SetupEnvCmd(in *Infra) error {
	if err := in.Validate(); err != nil {
		return failure.Wrap(err, "in.Validate failed")
	}

	// Make sure if the EmvCmd is not initialized to initialize it
	if in.EnvCmd == nil {
		in.EnvCmd = EnvCmd
	}

	// We always want the RunE to use this function
	in.EnvCmd.RunE = in.RunEnv

	in.ParentCmd.AddCommand(in.EnvCmd)

	if in.EnvExportCmd == nil {
		in.EnvExportCmd = EnvExportCmd
	}
	in.EnvExportCmd.RunE = in.RunEnvExport
	in.EnvCmd.AddCommand(EnvExportCmd)

	var eb EnvBind
	if err := Bind(in.EnvCmd, in.Viper, &eb); err != nil {
		return failure.Wrap(err, "Bind failed for (in.EnvBind)")
	}

	var ex EnvExportBind
	if err := Bind(in.EnvExportCmd, in.Viper, &ex); err != nil {
		return failure.Wrap(err, "Bind failed for (in.EnvExportBind)")
	}

	return nil
}

var EnvCmd = &cobra.Command{
	Use:   "env [FEATURE]",
	Short: "display environment variables for a lambda",
	Args:  cobra.MinimumNArgs(0),
}

var EnvExportCmd = &cobra.Command{
	Use:   "export",
	Short: "exports all env vars for this service",
	Args:  cobra.MinimumNArgs(0),
}

type EnvBind struct {
	NamesOnly bool `conf:"cli:names-only, cli-u: Only display the env var names for a given feature"`
}

type EnvConfig struct {
	CmdConfig
	EnvBind
}

type EnvExportConfig struct {
	CmdConfig
	EnvExportBind
}

type EnvExportBind struct {
	File      Filepath `conf:"cli:file, cli-s:f, cli-u:Export to a json file"`
	StdOut    bool     `conf:"cli:stdout, cli-u: Importing values from env vars on you machine"`
	NamesOnly bool     `conf:"cli:names-only, cli-u: Only display the env var names for a given feature"`
}

type EnvResult struct {
	FeatureName string
	Items       map[string]string
	Names       []string
	Display     string
}

func (i *Infra) RunEnvExport(cmd *cobra.Command, args []string) error {
	if err := i.Validate(); err != nil {
		return failure.Wrap(err, "i.ValidateInitialize")
	}
	var config EnvExportConfig

	if err := i.Process(cmd, &config); err != nil {
		return failure.Wrap(err, "i.Configure failed")
	}

	if config.IsAll {
		service, err := i.LoadService(config.CmdConfig)
		if err != nil {
			return failure.Wrap(err, "[env, report-all] i.LoadService failed")
		}

		result, invalid, err := i.ServiceEnvReport(service, config.CmdConfig, config.NamesOnly)
		if err != nil {
			return failure.Wrap(err, "i.ServiceEnvReport failed")
		}

		if len(invalid) > 0 {
			i.DisplayErrorJson(invalid)
		}

		if !config.File.IsEmpty() {
			i.WriteJson(config.File.Path, result)
			return nil
		}

		i.DisplayJson(result)
		return nil
	}

	if len(args) == 0 {
		return failure.InvalidParam("feature or --all flag is required")
	}

	_, feature, err := i.LoadFeature(config.CmdConfig, args[0])
	if err != nil {
		return failure.Wrap(err, "i.LoadFeatureFromFirstArg")
	}

	result, err := i.FeatureEnvReport(feature.Conf, config.CmdConfig, config.NamesOnly)
	if err != nil {
		return failure.Wrap(err, "i.FeatureEnvReport failed")
	}

	if !config.File.IsEmpty() {
		i.WriteJson(config.File.Path, result)
		return nil
	}

	i.DisplayJson(result)
	return nil
}

func (i *Infra) RunEnv(cmd *cobra.Command, args []string) error {
	if err := i.Validate(); err != nil {
		return failure.Wrap(err, "i.ValidateInitialize")
	}
	var config EnvConfig

	if err := i.Process(cmd, &config); err != nil {
		return failure.Wrap(err, "i.Configure failed")
	}

	if config.CmdConfig.IsAll {
		service, err := i.LoadService(config.CmdConfig)
		if err != nil {
			return failure.Wrap(err, "[env, report-all] i.LoadService failed")
		}

		result, invalid, err := i.ServiceEnvReport(service, config.CmdConfig, config.NamesOnly)
		if err != nil {
			return failure.Wrap(err, "i.ServiceEnvReport failed")
		}

		if len(invalid) > 0 {
			i.DisplayErrorJson(invalid)
		}

		i.DisplayJson(result)
		return nil
	}

	if len(args) == 0 {
		return failure.InvalidParam("feature or --all flag is required")
	}

	_, feature, err := i.LoadFeature(config.CmdConfig, args[0])
	if err != nil {
		return failure.Wrap(err, "i.LoadFeatureFromFirstArg")
	}

	result, err := i.FeatureEnvReport(feature.Conf, config.CmdConfig, config.NamesOnly)
	if err != nil {
		return failure.Wrap(err, "i.FeatureEnvReport failed")
	}

	i.DisplayJson(result)
	return nil
}

func (i *Infra) ServiceEnvReport(service *sls.MicroService, c CmdConfig, isNamesOnly ...bool) (map[string]string, map[string]map[string]string, error) {
	var result = map[string]string{}
	var invalid = map[string]map[string]string{}

	for title, feature := range service.Features {
		key := title
		if c.WithTrigger {
			key = feature.NameWithTrigger()
		}

		envs, err := i.FeatureEnvReport(feature.Conf, c, isNamesOnly...)
		if err != nil {
			return nil, nil, failure.Wrap(err, "i.FeatureEnvReport")
		}
		for env, value := range envs {
			existing, ok := result[env]
			if ok {
				if existing != value {
					invalid[key] = map[string]string{env: value}
				}
				continue
			} else {
				result[env] = value
			}
		}
	}
	return result, invalid, nil
}

func (i *Infra) FeatureEnvReport(config sls.Configurable, c CmdConfig, isNamesOnly ...bool) (map[string]string, error) {
	var result = map[string]string{}

	config.SetExcludeDefaults(c.SkipDefaults)
	if len(isNamesOnly) > 0 && isNamesOnly[0] {
		out, err := config.EnvNames()
		if err != nil {
			return nil, failure.Wrap(err, "config.EnvNames failed")
		}

		for _, k := range out {
			result[k] = ""
		}
		return result, nil
	}

	result, err := config.EnvToMap()
	if err != nil {
		return nil, failure.Wrap(err, "config.EnvToMap failed")
	}

	return result, nil
}

func EnvMap(feature sls.Feature, config EnvConfig) (*EnvResult, error) {
	if err := feature.Conf.ProcessEnv(); err != nil {
		return nil, failure.Wrap(err, "feature.Conf.ProcessEnv failed")
	}

	items, err := feature.Conf.EnvToMap()
	if err != nil {
		return nil, failure.Wrap(err, "feature.Conf.EnvToMap failed")
	}

	fName := feature.Name
	if config.IsQualifiedName {
		fName = feature.QualifiedName
	}

	display := map[string]map[string]string{
		fName: items,
	}

	result := EnvResult{
		FeatureName: fName,
		Items:       items,
	}

	if config.IsText {
		result.Display = fmt.Sprintf("%s\n", fName)
		for k, v := range items {
			result.Display += fmt.Sprintf("%s: %s\n", k, v)
		}
		return &result, nil
	}

	data, err := json.Marshal(display)
	if err != nil {
		return nil, failure.ToSystem(err, "json.Marshal failed for EnvMap")
	}
	result.Display = string(data)

	return &result, nil
}
