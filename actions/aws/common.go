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
	"encoding/json"
	"fmt"
	"strings"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
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
	InputExternalID    = "external_id"
)

// Credentials bundles the standard AWS auth inputs.
type Credentials struct {
	AccessKey     string
	SecretKey     string
	Region        string
	SessionToken  string // optional (temporary/STS credentials)
	AssumeRoleARN string // optional (cross-account / least-privilege)
	ExternalID    string // optional; paired with AssumeRoleARN to defeat the
	// confused-deputy problem — the value must match the External ID in the
	// target role's trust policy. Ignored unless AssumeRoleARN is set.
}

// Config builds an aws.Config for one of two authentication methods:
//
//   - Access Keys: the user supplies AccessKey/SecretKey (their own IAM keys).
//   - Assume Role: the user supplies only AssumeRoleARN (+ optional ExternalID).
//     The BASE identity that calls STS AssumeRole is then Flomation's own — taken
//     from the runner host's ambient AWS credential chain (the AWS_ACCESS_KEY_ID/
//     AWS_SECRET_ACCESS_KEY env of a dedicated Flomation IAM user, or an instance
//     role where available). The customer never enters Flomation's keys; they
//     grant Flomation's principal sts:AssumeRole on their role instead.
//
// Region is required, and at least one of AccessKey or AssumeRoleARN must be set.
// The returned config's credentials are cached and auto-refreshed by the SDK.
func Config(ctx context.Context, c Credentials) (awssdk.Config, error) {
	if c.Region == "" {
		return awssdk.Config{}, fmt.Errorf("aws region is required")
	}
	if c.AccessKey == "" && c.AssumeRoleARN == "" {
		return awssdk.Config{}, fmt.Errorf("provide AWS access keys, or a role ARN to assume")
	}

	opts := []func(*config.LoadOptions) error{config.WithRegion(c.Region)}
	// User-supplied static keys → use them as the base identity. Otherwise fall
	// through to the SDK default chain (Flomation's ambient identity), which is
	// only meaningful when assuming a role below.
	if c.AccessKey != "" && c.SecretKey != "" {
		opts = append(opts, config.WithCredentialsProvider(credentials.StaticCredentialsProvider{
			Value: awssdk.Credentials{
				AccessKeyID:     c.AccessKey,
				SecretAccessKey: c.SecretKey,
				SessionToken:    c.SessionToken,
				Source:          "Flomation AWS",
			},
		}))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return awssdk.Config{}, fmt.Errorf("load aws config: %w", err)
	}

	if c.AssumeRoleARN != "" {
		stsClient := sts.NewFromConfig(cfg)
		provider := stscreds.NewAssumeRoleProvider(stsClient, c.AssumeRoleARN, func(o *stscreds.AssumeRoleOptions) {
			if c.ExternalID != "" {
				o.ExternalID = awssdk.String(c.ExternalID)
			}
		})
		cfg.Credentials = awssdk.NewCredentialsCache(provider)
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
		ExternalID:    InputString(InputExternalID, inputs),
	})
}

// InputAccountID is the standard account-scope input for s3control and other
// account-scoped AWS actions.
const InputAccountID = "account_id"

// ResolveAccountID returns the 12-digit AWS account ID that account-scoped
// services (notably s3control) require on every call. If the account_id input
// is set it is used verbatim; otherwise it is derived from the active
// credentials via STS GetCallerIdentity, so users can leave the field blank and
// let Flomation infer it from whichever identity the action is running as
// (static keys, an assumed role, or a managed credential).
func ResolveAccountID(ctx context.Context, cfg awssdk.Config, inputs []*core.Connection) (string, error) {
	if id := strings.TrimSpace(InputString(InputAccountID, inputs)); id != "" {
		return id, nil
	}
	out, err := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", fmt.Errorf("could not resolve AWS account id via STS GetCallerIdentity (or set the Account ID input): %w", err)
	}
	return awssdk.ToString(out.Account), nil
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

// InputStrings reads a list-of-ids input robustly, accepting any of the shapes a
// value can arrive in: a native []string/[]interface{} (e.g. an array output
// wired directly), a JSON array string (e.g. ${node.instance_ids} substituted
// into this string input), or a plain comma-separated string. A JSON object
// wired in by mistake yields nil (so the action reports "id required" rather than
// passing a mangled fragment to AWS).
func InputStrings(name string, inputs []*core.Connection) []string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Value == nil {
		return nil
	}
	switch v := conn.Value.(type) {
	case []string:
		return trimList(v)
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, e := range v {
			out = append(out, fmt.Sprintf("%v", e))
		}
		return trimList(out)
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return nil
		}
		if strings.HasPrefix(s, "[") { // JSON array from a wired array output
			var arr []interface{}
			if err := json.Unmarshal([]byte(s), &arr); err == nil {
				out := make([]string, 0, len(arr))
				for _, e := range arr {
					out = append(out, fmt.Sprintf("%v", e))
				}
				return trimList(out)
			}
		}
		if strings.HasPrefix(s, "{") { // a JSON object wired by mistake — not a list
			return nil
		}
		return trimList(strings.Split(s, ","))
	}
	return nil
}

func trimList(in []string) []string {
	var out []string
	for _, p := range in {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// FilterSpec maps an action input to an EC2 filter name. Multi means the input
// holds several OR'd values (a multi-select or comma-separated list, read via
// InputStrings); otherwise it's a single string value. Blank/absent inputs are
// skipped, so an unset filter never constrains the query.
type FilterSpec struct {
	Input  string
	Filter string
	Multi  bool
}

// EC2TagFilters converts the standard "filter_tags" key-value-array input into
// EC2 tag filters: "tag:<Key>" = <Value>, or "tag-key" = <Key> when the value is
// blank (i.e. "has this tag key, any value"). EC2 ANDs all returned filters.
func EC2TagFilters(inputs []*core.Connection) []ec2types.Filter {
	var filters []ec2types.Filter
	conn := core.FindConnection("filter_tags", inputs)
	if conn == nil {
		return nil
	}
	for _, kv := range conn.KeyValuePairs() {
		k := strings.TrimSpace(kv.Key)
		if k == "" {
			continue
		}
		if v := strings.TrimSpace(kv.Value); v != "" {
			filters = append(filters, ec2types.Filter{Name: awssdk.String("tag:" + k), Values: []string{v}})
		} else {
			filters = append(filters, ec2types.Filter{Name: awssdk.String("tag-key"), Values: []string{k}})
		}
	}
	return filters
}

// BuildEC2Filters assembles the combined EC2 filter list for a Describe call: the
// standard tag filters (from "filter_tags") plus each named spec. It is the one
// place the tag/named/multi filter conventions live, so every AWS Describe action
// gets identical, tested behaviour by just declaring its relevant specs.
func BuildEC2Filters(inputs []*core.Connection, specs []FilterSpec) []ec2types.Filter {
	filters := EC2TagFilters(inputs)
	for _, s := range specs {
		if s.Multi {
			if vals := InputStrings(s.Input, inputs); len(vals) > 0 {
				filters = append(filters, ec2types.Filter{Name: awssdk.String(s.Filter), Values: vals})
			}
			continue
		}
		if v := strings.TrimSpace(InputString(s.Input, inputs)); v != "" {
			filters = append(filters, ec2types.Filter{Name: awssdk.String(s.Filter), Values: []string{v}})
		}
	}
	return filters
}
