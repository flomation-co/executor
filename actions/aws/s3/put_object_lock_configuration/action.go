// Package aws_s3_put_object_lock_configuration sets a bucket's Object Lock configuration.
package aws_s3_put_object_lock_configuration

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsS3 "github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS S3 Put Object Lock Configuration"
	Description  = "Enable Object Lock and set default WORM retention on an AWS S3 bucket."
	Website      = "https://www.flomation.co"
	Icon         = "lock+pen"
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
	{Name: "object_lock_enabled", Type: core.ConnectionTypeBoolean, Label: "Object Lock Enabled", Value: true},
	{Name: "default_mode", Type: core.ConnectionTypeString, Label: "Default Retention Mode (optional)", Options: []core.ConnectionOption{
		{Name: "Governance", Value: "GOVERNANCE"},
		{Name: "Compliance", Value: "COMPLIANCE"},
	}},
	{Name: "default_days", Type: core.ConnectionTypeInteger, Label: "Default Retention Days (optional)", Placeholder: "30"},
	{Name: "default_years", Type: core.ConnectionTypeInteger, Label: "Default Retention Years (optional)", Placeholder: "1"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "bucket", Type: core.ConnectionTypeString, Label: "Bucket"},
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

	config := &s3types.ObjectLockConfiguration{
		ObjectLockEnabled: s3types.ObjectLockEnabledEnabled,
	}

	mode := awscommon.InputString("default_mode", inputs)
	if mode != "" {
		retention := &s3types.DefaultRetention{
			Mode: s3types.ObjectLockRetentionMode(mode),
		}
		days, hasDays := awscommon.InputInt("default_days", inputs)
		years, hasYears := awscommon.InputInt("default_years", inputs)
		if hasDays && hasYears {
			return nil, fmt.Errorf("specify either default_days or default_years, not both")
		}
		if hasDays {
			retention.Days = aws.Int32(int32(days))
		} else if hasYears {
			retention.Years = aws.Int32(int32(years))
		} else {
			return nil, fmt.Errorf("default_days or default_years is required when a default retention mode is set")
		}
		config.Rule = &s3types.ObjectLockRule{DefaultRetention: retention}
	}

	_, err = client.PutObjectLockConfiguration(ctx, &awsS3.PutObjectLockConfigurationInput{
		Bucket:                  aws.String(bucket),
		ObjectLockConfiguration: config,
	})
	if err != nil {
		return nil, err
	}

	summary := fmt.Sprintf("Object Lock enabled on bucket %s", bucket)
	if mode != "" {
		summary = fmt.Sprintf("%s with default %s retention", summary, mode)
	}
	return map[string]interface{}{
		"tool_result": summary,
		"bucket":      bucket,
	}, nil
}
