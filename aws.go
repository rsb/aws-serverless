package sls

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/rsb/failure"
)

func NewDefaultConfig(opts ...context.Context) (config.Config, error) {
	ctx := resolveContextOpts(opts...)

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return cfg, failure.ToConfig(err, "config.LoadDefaultConfig failed")
	}

	return cfg, nil
}

func NewDefaultConfigMust(opts ...context.Context) config.Config {
	cfg, err := NewDefaultConfig(opts...)
	if err != nil {
		panic(failure.Wrap(err, "NewDefaultConfig failed"))
	}

	return cfg
}

func NewDefaultConfigWithRegion(reg string, opts ...context.Context) (config.Config, error) {
	var cfg config.Config

	ctx := resolveContextOpts(opts...)
	region, err := ToRegion(reg)
	if err != nil {
		return cfg, failure.Wrap(err, "ToRegion failed")
	}

	cfg, err = config.LoadDefaultConfig(ctx, config.WithRegion(region.String()))
	if err != nil {
		return cfg, failure.Wrap(err, "config.LoadDefaultConfig failed")
	}

	return cfg, nil
}

func resolveContextOpts(opts ...context.Context) context.Context {
	var ctx context.Context

	if len(opts) > 0 && opts[0] != nil {
		ctx = opts[0]
	} else {
		ctx = context.TODO()
	}

	return ctx
}
