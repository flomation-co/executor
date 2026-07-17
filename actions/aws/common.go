// Package aws holds shared helpers for every AWS action. It has no Execute
// function, so the manifest generator skips it (like sql/common.go), but its
// category.go still supplies the top-level "AWS" category metadata.
//
// The one thing every AWS action needs is an aws.Config built from the user's
// credentials. Config/ConfigFromInputs centralise that so the ~dozens of service
// actions don't each re-implement credential loading. Note the manifest
// generator only resolves INLINE Inputs literals, so the credential *input
// declarations* (aws_access_key, aws_secret_key, aws_region, ...) must still be
// copy-pasted into each action's Inputs — only the Execute-side logic is shared.
package aws

import (
	"context"
	"fmt"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	core "flomation.app/automate/executor"
)

// Standard input names shared by every AWS action.
const (
	InputAccessKey     = "aws_access_key"
	InputSecretKey     = "aws_secret_key"
	InputRegion        = "aws_region"
	InputSessionToken  = "aws_session_token"
	InputAssumeRoleARN = "assume_role_arn"
)

// Credentials bundles the standard AWS auth inputs.
type Credentials struct {
	AccessKey     string
	SecretKey     string
	Region        string
	SessionToken  string // optional (temporary/STS credentials)
	AssumeRoleARN string // optional (cross-account / least-privilege)
}

// Config builds an aws.Config from static credentials, optionally assuming a
// role. Region is required. When AssumeRoleARN is set, the static credentials
// are used only to call STS AssumeRole and the returned config carries the
// assumed-role credentials (cached + auto-refreshed).
func Config(ctx context.Context, c Credentials) (awssdk.Config, error) {
	if c.Region == "" {
		return awssdk.Config{}, fmt.Errorf("aws region is required")
	}
	if c.AccessKey == "" || c.SecretKey == "" {
		return awssdk.Config{}, fmt.Errorf("aws access key and secret key are required")
	}

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(c.Region),
		config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: awssdk.Credentials{
				AccessKeyID:     c.AccessKey,
				SecretAccessKey: c.SecretKey,
				SessionToken:    c.SessionToken,
				Source:          "Flomation AWS",
			},
		}),
	)
	if err != nil {
		return awssdk.Config{}, fmt.Errorf("load aws config: %w", err)
	}

	if c.AssumeRoleARN != "" {
		stsClient := sts.NewFromConfig(cfg)
		cfg.Credentials = awssdk.NewCredentialsCache(
			stscreds.NewAssumeRoleProvider(stsClient, c.AssumeRoleARN),
		)
	}

	return cfg, nil
}

// ConfigFromInputs reads the standard credential input block and builds an
// aws.Config. Every action's Execute calls this.
func ConfigFromInputs(ctx context.Context, inputs []*core.Connection) (awssdk.Config, error) {
	return Config(ctx, Credentials{
		AccessKey:     InputString(InputAccessKey, inputs),
		SecretKey:     InputString(InputSecretKey, inputs),
		Region:        InputString(InputRegion, inputs),
		SessionToken:  InputString(InputSessionToken, inputs),
		AssumeRoleARN: InputString(InputAssumeRoleARN, inputs),
	})
}

// InputString returns a string input's value, or "" when absent/unset.
func InputString(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return ""
	}
	if s := c.String(); s != nil {
		return *s
	}
	return ""
}

// InputStrings splits a comma-separated input into a trimmed, non-empty slice.
// Handy for the many EC2/RDS calls that take a list of ids.
func InputStrings(name string, inputs []*core.Connection) []string {
	raw := InputString(name, inputs)
	if raw == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
