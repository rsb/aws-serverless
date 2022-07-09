package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/rsb/failure"
	"github.com/rsb/sls"
	"github.com/spf13/cobra"
)

func SetupParamStoreCmd(in *Infra) error {
	if err := in.Validate(); err != nil {
		return failure.Wrap(err, "in.Validate failed")
	}

	// Make sure if the EmvCmd is not initialized to initialize it
	if in.PStoreCmd == nil {
		in.PStoreCmd = PStoreCmd
	}
	in.PStoreCmd.RunE = in.RunPStore
	in.ParentCmd.AddCommand(in.PStoreCmd)

	if in.PStoreImportCmd == nil {
		in.PStoreImportCmd = PStoreImportCmd
	}
	in.PStoreImportCmd.RunE = in.RunPStoreImport
	in.PStoreCmd.AddCommand(in.PStoreImportCmd)

	if in.PStoreExportCmd == nil {
		in.PStoreExportCmd = PStoreExportCmd
	}
	in.PStoreExportCmd.RunE = in.RunPStoreExport
	in.PStoreCmd.AddCommand(in.PStoreExportCmd)

	if in.PStoreDeleteCmd == nil {
		in.PStoreDeleteCmd = PStoreDeleteCmd
	}
	in.PStoreDeleteCmd.RunE = in.RunPStoreDelete
	in.PStoreCmd.AddCommand(in.PStoreDeleteCmd)

	var pa PStoreImportBind
	if err := Bind(in.PStoreImportCmd, in.Viper, &pa); err != nil {
		return failure.Wrap(err, "Bind failed for in.PStoreImportCmd")
	}

	var pe PStoreExportBind
	if err := Bind(in.PStoreExportCmd, in.Viper, &pe); err != nil {
		return failure.Wrap(err, "Bind failed for in.PStoreExportCmd")
	}

	return nil
}

var PStoreCmd = &cobra.Command{
	Use:   "pstore [FEATURE]",
	Short: "display parameter store values for a lambda",
	Args:  cobra.MinimumNArgs(0),
}

var PStoreImportCmd = &cobra.Command{
	Use:   "import",
	Short: "import params into parameter store",
}

var PStoreExportCmd = &cobra.Command{
	Use:   "export",
	Short: "exports all params for this service",
	Args:  cobra.MinimumNArgs(0),
}

var PStoreDeleteCmd = &cobra.Command{
	Use:   "delete <PARAM_NAME>",
	Short: "delete single param or all params from the micro-service",
	Args:  cobra.MaximumNArgs(1),
}

type PStoreDeleteBind struct {
	IsAll     bool   `conf:"cli:all, cli-s: a, cli-u: delete all parameters for this micro-service"`
	IsEncrypt bool   `conf:"default:true, cli:encrypt, cli-u: use encryption for pstore"`
	Feature   string `conf:"cli:feature, cli-u: delete only params for the given feature"`
}

type PStoreDeleteConfig struct {
	CmdConfig
	PStoreDeleteBind
}

type PStoreImportBind struct {
	File          Filepath `conf:"cli:file, cli-s:f, cli-u:Importing values from a json file"`
	ImportFromEnv bool     `conf:"cli:env, cli-u: Importing values from env vars on you machine"`
	IsEncrypt     bool     `conf:"default:true, cli:encrypt, cli-u: use encryption for pstore"`
	Overwrite     bool     `conf:"cli:overwrite, cli-u: Used to replace values that already exist in parameter store"`
}

type PStoreExportBind struct {
	File      Filepath `conf:"cli:file, cli-s:f, cli-u:Export to a json file"`
	IsEncrypt bool     `conf:"default:true, cli:encrypt, cli-u: use encryption for pstore"`
	StdOut    bool     `conf:"cli:stdout, cli-u: Importing values from env vars on you machine"`
}

type PStoreBind struct {
	IsEncrypt bool `conf:"default:true, cli:encrypt, cli-u: use encryption for pstore"`
}

type PStoreConfig struct {
	CmdConfig
	PStoreBind
}

type PStoreImportConfig struct {
	CmdConfig
	PStoreImportBind
}

type PStoreExportConfig struct {
	CmdConfig
	PStoreExportBind
}

// RunPStore runs `<service> infra pstore` which will return parameter
// values for the service or feature
// `<service> infra pstore <FEATURE [-a --all]>` - will list all params for just that feature
func (i *Infra) RunPStore(cmd *cobra.Command, args []string) error {
	if err := i.Validate(); err != nil {
		return failure.Wrap(err, "i.Validate failed")
	}

	if i.PStoreAPI == nil {
		return failure.System("i.PStoreAPI is not initialized")
	}

	ctx := context.Background()
	var config PStoreConfig
	if err := i.Process(cmd, &config); err != nil {
		return failure.Wrap(err, "i.Configure failed")
	}

	if config.CmdConfig.IsAll {
		service, err := i.LoadService(config.CmdConfig)
		if err != nil {
			return failure.Wrap(err, "i.LoadService failed")
		}

		result, err := i.ServiceParams(ctx, service.Name.AppTitle())
		if err != nil {
			return failure.Wrap(err, "i.ServiceParams failed")
		}

		i.DisplayJson(result)
		return nil
	}

	if len(args) == 0 || args[0] == "" {
		return failure.System("parameter name is missing")
	}

	service, feature, err := i.LoadFeature(config.CmdConfig, args[0])
	if err != nil {
		return failure.Wrap(err, "i.LoadFeatureFromFirstArg")
	}

	result, err := i.FeatureParams(ctx, service.Name.AppTitle(), feature)
	if err != nil {
		return failure.Wrap(err, "i.FeatureParams")
	}

	i.DisplayJson(result)
	return nil
}

// infra env export --

// RunPStoreExport runs `<service> infra pstore export` which will return parameter
// values for the service or feature
// 1) `<service> infra pstore export`
// 2) `<service> infra pstore export [-f --file]`
// 3) `<service> infra pstore <FEATURE> [-f --file]` - export for that feature
func (i *Infra) RunPStoreExport(cmd *cobra.Command, args []string) error {
	if err := i.Validate(); err != nil {
		return failure.Wrap(err, "i.Validate failed")
	}
	if i.PStoreAPI == nil {
		return failure.System("i.PStoreAPI is not initialized")
	}
	ctx := context.Background()

	var config PStoreExportConfig
	if err := i.Process(cmd, &config); err != nil {
		return failure.Wrap(err, "i.Configure failed")
	}

	if config.IsAll {
		service, err := i.LoadService(config.CmdConfig)
		if err != nil {
			return failure.Wrap(err, "i.LoadService failed")
		}

		appTitle := service.Name.AppTitle()
		result, err := i.ServiceParams(ctx, appTitle)
		if err != nil {
			return failure.Wrap(err, "i.ServiceParams failed")
		}

		result = i.StripAppTitle(appTitle, result)
		if !config.File.IsEmpty() {
			i.WriteJson(config.File.Path, result)
			return nil
		}

		i.DisplayJson(result)
		return nil
	}

	if len(args) == 0 || args[0] == "" {
		return failure.System("parameter name is missing")
	}

	service, feature, err := i.LoadFeature(config.CmdConfig, args[0])
	if err != nil {
		return failure.Wrap(err, "i.LoadFeatureFromFirstArg")
	}

	appTitle := service.Name.AppTitle()
	result, err := i.FeatureParams(ctx, appTitle, feature)
	if err != nil {
		return failure.Wrap(err, "i.FeatureParams")
	}

	result = i.StripAppTitle(appTitle, result)
	if !config.File.IsEmpty() {
		i.WriteJson(config.File.Path, result)
		return nil
	}

	i.DisplayJson(result)
	return nil
}

// RunPStoreDelete runs `<service> infra pstore delete` which will return parameter
// values for the service or feature
// `<service> infra pstore delete <[FEATURE] | [--all]>`
func (i *Infra) RunPStoreDelete(cmd *cobra.Command, args []string) error {
	if err := i.Validate(); err != nil {
		return failure.Wrap(err, "i.Validate failed")
	}
	if i.PStoreAPI == nil {
		return failure.System("i.PStoreAPI is not initialized")
	}

	var config PStoreDeleteConfig
	if err := i.Process(cmd, &config); err != nil {
		return failure.Wrap(err, "i.Configure failed")
	}

	ctx := context.Background()

	if config.CmdConfig.IsAll {
		if config.Feature == "" {
			service, err := i.LoadService(config.CmdConfig)
			if err != nil {
				return failure.Wrap(err, "i.LoadService failed")
			}

			result, err := i.DeleteAllServiceParams(ctx, service.Name.AppTitle())
			if err != nil {
				return failure.Wrap(err, "i.DeleteAllServiceParams failed")
			}

			i.DisplayJson(result)
		} else {
			service, feature, err := i.LoadFeature(config.CmdConfig, args[0])
			if err != nil {
				return failure.Wrap(err, "i.LoadFeatureFromFirstArg")
			}
			result, err := i.DeleteAllFeatureParams(ctx, service.Name.AppTitle(), feature)
			if err != nil {
				return failure.Wrap(err, "i.DeleteAllFeatureParams failed")
			}

			i.DisplayJson(result)
		}
		return nil
	}

	if len(args) == 0 || args[0] == "" {
		return failure.System("parameter name is missing")
	}

	service, err := i.LoadService(config.CmdConfig)
	if err != nil {
		return failure.Wrap(err, "i.LoadService failed")
	}

	appTitle := service.Name.AppTitle()
	result, err := i.DeleteParam(ctx, appTitle, args[0])

	i.DisplayJson(result)
	return nil
}

// RunPStoreImport runs `<service> infra pstore import` which will import all parameters
// `<service> infra pstore import <[--file | --env]>`
func (i *Infra) RunPStoreImport(cmd *cobra.Command, _ []string) error {
	if err := i.Validate(); err != nil {
		return failure.Wrap(err, "i.ValidateInitialize")
	}
	if i.PStoreAPI == nil {
		return failure.System("i.PStoreAPI is not initialized")
	}
	var config PStoreImportConfig

	if err := i.Process(cmd, &config); err != nil {
		return failure.Wrap(err, "i.Configure failed")
	}

	service, err := i.LoadService(config.CmdConfig)
	if err != nil {
		return failure.Wrap(err, "i.LoadService failed")
	}

	appTitle := service.Name.AppTitle()

	params := map[string]string{}
	if !config.File.IsEmpty() {
		params, err = readPStoreParamsFromFile(appTitle, config.File.Path)
		if err != nil {
			return failure.Wrap(err, "readPStoreParamsFromFile failed")
		}
	} else {
		params, err = readPStoreParamsFromService(service)
		if err != nil {
			return failure.Wrap(err, "readPStorePramsFromService failed")
		}
	}

	var errs []error
	ctx := context.Background()
	backup := map[string]string{}
	overwrite := config.Overwrite
	for k, v := range params {
		old, err := i.PutParam(ctx, appTitle, k, v, overwrite)
		if err != nil && failure.IsNotFound(err) {
			errs = append(errs, err)
			continue
		}

		// Add old values to be sent back
		for bk, bv := range old {
			backup[bk] = bv
		}
	}

	if len(errs) > 0 {
		_, _ = fmt.Fprintf(i.Stderr, "%+v\n", errs)
	}

	i.DisplayJson(backup)
	return nil
}

func readPStoreParamsFromService(service *sls.MicroService) (map[string]string, error) {
	params := map[string]string{}
	appTitle := service.Name.AppTitle()
	for title, feature := range service.Features {
		if feature.Conf == nil {
			return nil, failure.System("[%s] feature.Conf is nil, not initialized", title)
		}
		result, err := feature.Conf.CollectParamsFromEnv(appTitle)
		if err != nil {
			return params, failure.Wrap(err, "feature.Conf.ProcessParamStore failed (%s)", title)
		}

		for k, v := range result {
			existing, ok := params[k]
			if ok {
				if existing != v {
					return params, failure.System("params (%s, %v) has two different values (%s, %s)", k, existing, v, feature.Name)
				}
				continue
			}
			params[k] = v
		}
	}
	return params, nil
}

func readPStoreParamsFromFile(appTitle, f string) (map[string]string, error) {
	file, err := ioutil.ReadFile(f)
	if err != nil {
		return nil, failure.Wrap(err, "ioutil.ReadFile failed, (%s)", f)
	}

	var data map[string]string
	if err = json.Unmarshal(file, &data); err != nil {
		return nil, failure.Wrap(err, "json.Unmarshal failed")
	}

	return data, nil
}

func (i *Infra) Param(ctx context.Context, appTitle, key string) (string, error) {
	if appTitle == "" {
		return "", failure.System("appTitle is empty. should be the terraform name of the mico-service")
	}

	if !strings.HasPrefix(key, appTitle) {
		key = fmt.Sprintf("%s/%s", appTitle, key)
	}

	path := i.PStoreAPI.EnsurePathPrefix(key)

	value, err := i.PStoreAPI.Param(ctx, path)
	if err != nil {
		return "", failure.Wrap(err, "i.PStoreAPI.Param failed (%s, %s)", appTitle, key)
	}

	return value, nil
}

func (i *Infra) PutParam(ctx context.Context, appTitle, key, value string, overwrite bool) (map[string]string, error) {
	if appTitle == "" {
		return nil, failure.System("appTitle is empty. should be the terraform name of the mico-service")
	}

	if !strings.HasPrefix(key, appTitle) {
		key = fmt.Sprintf("%s/%s", appTitle, key)
	}

	path := i.PStoreAPI.EnsurePathPrefix(key)

	old, err := i.PStoreAPI.Put(ctx, path, value, overwrite)
	if err != nil {
		return nil, failure.Wrap(err, "i.PStoreAPI.Param failed (%s, %s)", appTitle, key)
	}

	return map[string]string{path: old}, nil
}

func (i *Infra) DeleteParam(ctx context.Context, appTitle, key string) (map[string]string, error) {
	if !strings.HasPrefix(key, appTitle) {
		key = fmt.Sprintf("%s/%s", appTitle, key)
	}

	path := i.PStoreAPI.EnsurePathPrefix(key)
	value, err := i.PStoreAPI.Delete(ctx, path)
	if err != nil {
		return nil, failure.Wrap(err, "i.PStoreAPI.Delete failed (%s, %s)", appTitle, key)
	}

	result := map[string]string{
		path: value,
	}
	return result, nil
}

func (i *Infra) DeleteAllFeatureParams(ctx context.Context, appTitle string, feature sls.Feature) (map[string]string, error) {
	if appTitle == "" {
		return nil, failure.System("appTitle is empty. should be the terraform name of the mico-service")
	}

	result, err := i.FeatureParams(ctx, appTitle, feature)
	if err != nil {
		return nil, failure.Wrap(err, "i.FeatureParams failed for (%s)", appTitle)
	}

	for key := range result {

		if _, err := i.PStoreAPI.Delete(ctx, key); err != nil {
			return result, failure.ToSystem(err, "i.PStoreAPI.Delete failed")
		}
	}

	return result, nil
}

func (i *Infra) DeleteAllServiceParams(ctx context.Context, appTitle string) (map[string]string, error) {
	if appTitle == "" {
		return nil, failure.System("appTitle is empty. should be the terraform name of the mico-service")
	}

	result, err := i.ServiceParams(ctx, appTitle)
	if err != nil {
		return nil, failure.Wrap(err, "pstoreGetAllFromService failed for (%s)", appTitle)
	}

	for key := range result {

		if _, err := i.PStoreAPI.Delete(ctx, key); err != nil {
			return result, failure.ToSystem(err, "i.PStoreAPI.Delete failed")
		}
	}

	return result, nil
}

func (i *Infra) ServiceParams(ctx context.Context, appTitle string) (map[string]string, error) {
	if appTitle == "" {
		return nil, failure.System("appTitle is empty. should be the terraform name of the mico-service")
	}

	result, err := i.PStoreAPI.Path(ctx, appTitle)
	if err != nil {
		return nil, failure.Wrap(err, "i.PStoreAPI.Path failed")
	}

	return result, nil
}

func (i *Infra) FeatureParams(ctx context.Context, appTitle string, feature sls.Feature) (map[string]string, error) {
	if appTitle == "" {
		return nil, failure.System("appTitle is empty. should be the terraform name of the mico-service")
	}

	config := feature.Conf
	config.MarkDefaultsAsExcluded()
	keys, err := feature.Conf.EnvNames()
	if err != nil {
		return nil, failure.Wrap(err, "feature.Conf.EnvNames failed for (%s, %s)", appTitle, feature.Name)
	}

	var result = map[string]string{}
	for _, key := range keys {
		key = fmt.Sprintf("/%s/%s", appTitle, key)
		value, err := i.PStoreAPI.Param(ctx, key)
		if err != nil {
			return nil, failure.Wrap(err, "i.PStoreAPI.Param failed (%s, %s, %s)", appTitle, feature.Name, key)
		}
		result[key] = value
	}

	return result, nil
}

func (i *Infra) StripAppTitle(appTitle string, in map[string]string) map[string]string {
	appTitle = "/" + appTitle + "/"
	var out = map[string]string{}
	for k, v := range in {
		k = strings.Replace(k, appTitle, "", 1)
		out[k] = v
	}

	return out
}
