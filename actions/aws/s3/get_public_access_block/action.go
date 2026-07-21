// Package aws_s3_get_public_access_block reads an AWS S3 bucket's public access block config.
package aws_s3_get_public_access_block

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsS3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS S3 Get Public Access Block"
	Description  = "Read the public access block configuration of an AWS S3 bucket."
	Website      = "https://www.flomation.co"
	Icon         = "bucket+shield-halved"
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
	{Name: "bucket", Type: core.ConnectionTypeString, Label: "Bucket Name", Placeholder: "my-bucket", Required: true},
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

	bucket := awscommon.InputString("bucket", inputs)
	if bucket == "" {
		return nil, fmt.Errorf("bucket is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := awsS3.NewFromConfig(cfg)

	out, err := client.GetPublicAccessBlock(ctx, &awsS3.GetPublicAccessBlockInput{Bucket: aws.String(bucket)})
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
		"tool_result":             fmt.Sprintf("Public access block for %s: block_acls=%t ignore_acls=%t block_policy=%t restrict=%t", bucket, blockACLs, ignoreACLs, blockPolicy, restrict),
		"block_public_acls":       blockACLs,
		"ignore_public_acls":      ignoreACLs,
		"block_public_policy":     blockPolicy,
		"restrict_public_buckets": restrict,
	}, nil
}
