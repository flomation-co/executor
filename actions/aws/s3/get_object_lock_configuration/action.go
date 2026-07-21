// Package aws_s3_get_object_lock_configuration reads a bucket's Object Lock configuration.
package aws_s3_get_object_lock_configuration

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
	Name         = "AWS S3 Get Object Lock Configuration"
	Description  = "Read the Object Lock and default WORM retention of an AWS S3 bucket."
	Website      = "https://www.flomation.co"
	Icon         = "lock+magnifying-glass"
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
	{Name: "bucket", Type: core.ConnectionTypeString, Label: "Bucket", Placeholder: "my-bucket", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "object_lock_enabled", Type: core.ConnectionTypeBoolean, Label: "Object Lock Enabled"},
	{Name: "default_mode", Type: core.ConnectionTypeString, Label: "Default Retention Mode"},
	{Name: "default_days", Type: core.ConnectionTypeInteger, Label: "Default Retention Days"},
	{Name: "default_years", Type: core.ConnectionTypeInteger, Label: "Default Retention Years"},
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

	out, err := client.GetObjectLockConfiguration(ctx, &awsS3.GetObjectLockConfigurationInput{Bucket: aws.String(bucket)})
	if err != nil {
		return nil, err
	}

	var enabled bool
	var mode string
	var days, years int32
	if c := out.ObjectLockConfiguration; c != nil {
		enabled = c.ObjectLockEnabled == "Enabled"
		if c.Rule != nil && c.Rule.DefaultRetention != nil {
			dr := c.Rule.DefaultRetention
			mode = string(dr.Mode)
			days = aws.ToInt32(dr.Days)
			years = aws.ToInt32(dr.Years)
		}
	}

	return map[string]interface{}{
		"tool_result":         fmt.Sprintf("Bucket %s Object Lock enabled: %t", bucket, enabled),
		"object_lock_enabled": enabled,
		"default_mode":        mode,
		"default_days":        int64(days),
		"default_years":       int64(years),
	}, nil
}
