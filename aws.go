package sls

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/rsb/failure"
)

// AWSConfig is a configuration struct using the conf annotated tags which allows you to use
// it to configure cli systems or lambdas from env or cli inputs
type AWSConfig struct {
	Profile string `conf:"no-prefix, global-flag, 									 env:AWS_PROFILE,  cli:aws-profile, cli-u: aws profile"`
	Region  string `conf:"no-prefix, global-flag, default:us-east-1, env:AWS_REGION,	 cli:aws-region,  cli-u: aws region (default us-east-1)"`
}

type RegionConfig Region

func (c AWSConfig) ToRegion() (Region, error) {
	reg, err := ToRegion(c.Region)
	if err != nil {
		return "", failure.Wrap(err, "ToRegion failed")
	}

	return reg, nil
}

// NewDefaultConfigWithConf will use the config.LoadDefaultConfig with various
// different options set by the conf package. An Empty struct will give you the
// default config.
func NewDefaultConfigWithConf(c AWSConfig, opts ...context.Context) (aws.Config, error) {
	var cfg aws.Config
	var err error
	ctx := resolveContextOpts(opts...)

	var o []func(*config.LoadOptions) error

	if c.Profile != "" {
		o = append(o, config.WithSharedConfigProfile(c.Profile))
	}

	if c.Region != "" {
		reg, err := c.ToRegion()
		if err != nil {
			return cfg, failure.Wrap(err, "c.ToRegion failed")
		}

		o = append(o, config.WithRegion(reg.String()))
	}

	if len(o) == 0 {
		cfg, err = config.LoadDefaultConfig(ctx)
		if err != nil {
			return cfg, failure.Wrap(err, "NewConfig failed, (no region, no profile)")
		}
	}

	cfg, err = config.LoadDefaultConfig(ctx, o...)
	if err != nil {
		return cfg, failure.Wrap(err, "config.LoadDefaultConfig failed")
	}

	return cfg, nil
}

// NewDefaultConfig will return an aws.Config struct that can be used to configure
// most sdk clients. If no context is given this will create one
func NewDefaultConfig(opts ...context.Context) (aws.Config, error) {
	ctx := resolveContextOpts(opts...)

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return cfg, failure.ToConfig(err, "config.LoadDefaultConfig failed")
	}

	return cfg, nil
}

// NewDefaultConfigMust is the same as its cousin accept it will panic if it
// fails. This is useful in setup scenarios where you can not return any errors
func NewDefaultConfigMust(opts ...context.Context) aws.Config {
	cfg, err := NewDefaultConfig(opts...)
	if err != nil {
		panic(failure.Wrap(err, "NewDefaultConfig failed"))
	}

	return cfg
}

// NewDefaultConfigWithRegion just adds a aws region to the configuration.
// NOTE: the region is validated and will fail if not a proper aws region
func NewDefaultConfigWithRegion(reg string, opts ...context.Context) (aws.Config, error) {
	var cfg aws.Config

	ctx := resolveContextOpts(opts...)
	region, err := ToRegion(reg)
	if err != nil {
		return cfg, failure.Wrap(err, "ToRegion failed")
	}

	cfg, err = config.LoadDefaultConfig(ctx, config.WithRegion(region.String()))
	if err != nil {
		return cfg, failure.Wrap(err, "config.LoadDefaultConfig failed (region: %s)", region)
	}

	return cfg, nil
}

// NewDefaultConfigWithProfile just adds an aws profile to the configuration.
func NewDefaultConfigWithProfile(profile string, opts ...context.Context) (aws.Config, error) {
	var cfg aws.Config

	ctx := resolveContextOpts(opts...)
	cfg, err := config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(profile))
	if err != nil {
		return cfg, failure.Wrap(err, "config.LoadDefaultConfig failed (profile: %s)", profile)
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
