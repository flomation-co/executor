// Package aws_s3_get_account_public_access_block reads the account-level S3
// public access block configuration (via the s3control SDK).
package aws_s3_get_account_public_access_block

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3control"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS S3 Get Account Public Access Block"
	Description  = "Read the account-level S3 public access block configuration."
	Website      = "https://www.flomation.co"
	Icon         = "shield-halved+magnifying-glass"
	Date         = "21/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Required: true, Options: []core.ConnectionOption{
		{Name: "Access Keys", Value: "keys"},
		{Name: "Assume Role (cross-account)", Value: "assume_role"},
		{Name: "Managed Role (Credential)", Value: "credential"},
	}},
	{Name: "aws_access_key", Type: core.ConnectionTypeSecret, Label: "AWS Access Key", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "aws_secret_key", Type: core.ConnectionTypeSecret, Label: "AWS Secret Key", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "aws_region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "eu-west-2", Required: true},
	{Name: "aws_session_token", Type: core.ConnectionTypeSecret, Label: "Session Token (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "assume_role_arn", Type: core.ConnectionTypeString, Label: "Role ARN to Assume", Placeholder: "arn:aws:iam::<your-account>:role/FlomationAccess", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"assume_role"}}},
	{Name: "external_id", Type: core.ConnectionTypeString, Label: "Assume Role External ID (optional)", Placeholder: "Must match the External ID in the role's trust policy", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"assume_role"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "AWS Role Credential", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"credential"}}},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "AWS Account ID", Placeholder: "12-digit account ID; leave blank to auto-detect from the credential"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "block_public_acls", Type: core.ConnectionTypeBoolean, Label: "Block Public ACLs"},
	{Name: "ignore_public_acls", Type: core.ConnectionTypeBoolean, Label: "Ignore Public ACLs"},
	{Name: "block_public_policy", Type: core.ConnectionTypeBoolean, Label: "Block Public Policy"},
	{Name: "restrict_public_buckets", Type: core.ConnectionTypeBoolean, Label: "Restrict Public Buckets"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}

	accountID, err := awscommon.ResolveAccountID(ctx, cfg, inputs)
	if err != nil {
		return nil, err
	}

	client := s3control.NewFromConfig(cfg)

	out, err := client.GetPublicAccessBlock(ctx, &s3control.GetPublicAccessBlockInput{
		AccountId: aws.String(accountID),
	})
	if err != nil {
		return nil, err
	}

	cfgOut := out.PublicAccessBlockConfiguration
	var blockACLs, ignoreACLs, blockPolicy, restrict bool
	if cfgOut != nil {
		blockACLs = aws.ToBool(cfgOut.BlockPublicAcls)
		ignoreACLs = aws.ToBool(cfgOut.IgnorePublicAcls)
		blockPolicy = aws.ToBool(cfgOut.BlockPublicPolicy)
		restrict = aws.ToBool(cfgOut.RestrictPublicBuckets)
	}

	return map[string]interface{}{
		"tool_result":             fmt.Sprintf("Account %s public access block: block_public_acls=%t, ignore_public_acls=%t, block_public_policy=%t, restrict_public_buckets=%t", accountID, blockACLs, ignoreACLs, blockPolicy, restrict),
		"block_public_acls":       blockACLs,
		"ignore_public_acls":      ignoreACLs,
		"block_public_policy":     blockPolicy,
		"restrict_public_buckets": restrict,
	}, nil
}
