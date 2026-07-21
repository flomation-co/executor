// Package aws_s3_put_object_retention sets an object's WORM retention.
package aws_s3_put_object_retention

import (
	"context"
	"fmt"
	"time"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsS3 "github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS S3 Put Object Retention"
	Description  = "Apply a WORM retention mode and retain-until date to an AWS S3 object."
	Website      = "https://www.flomation.co"
	Icon         = "lock+clock"
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
	{Name: "key", Type: core.ConnectionTypeString, Label: "Object Key", Placeholder: "path/to/object", Required: true},
	{Name: "mode", Type: core.ConnectionTypeString, Label: "Retention Mode", Required: true, Options: []core.ConnectionOption{
		{Name: "Governance", Value: "GOVERNANCE"},
		{Name: "Compliance", Value: "COMPLIANCE"},
	}},
	{Name: "retain_until", Type: core.ConnectionTypeString, Label: "Retain Until (RFC3339)", Placeholder: "2027-01-01T00:00:00Z", Required: true},
	{Name: "bypass_governance_retention", Type: core.ConnectionTypeBoolean, Label: "Bypass Governance Retention"},
	{Name: "version_id", Type: core.ConnectionTypeString, Label: "Version ID (optional)", Placeholder: "Leave blank for latest version"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "bucket", Type: core.ConnectionTypeString, Label: "Bucket"},
	{Name: "key", Type: core.ConnectionTypeString, Label: "Object Key"},
	{Name: "mode", Type: core.ConnectionTypeString, Label: "Retention Mode"},
	{Name: "retain_until", Type: core.ConnectionTypeString, Label: "Retain Until"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	bucket := awscommon.InputString("bucket", inputs)
	if bucket == "" {
		return nil, fmt.Errorf("bucket is required")
	}
	key := awscommon.InputString("key", inputs)
	if key == "" {
		return nil, fmt.Errorf("object key is required")
	}
	mode := awscommon.InputString("mode", inputs)
	if mode == "" {
		return nil, fmt.Errorf("retention mode is required")
	}
	retainUntil := awscommon.InputString("retain_until", inputs)
	if retainUntil == "" {
		return nil, fmt.Errorf("retain_until is required")
	}
	retainTime, err := time.Parse(time.RFC3339, retainUntil)
	if err != nil {
		return nil, fmt.Errorf("retain_until must be an RFC3339 timestamp (e.g. 2027-01-01T00:00:00Z): %w", err)
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := awsS3.NewFromConfig(cfg)

	in := &awsS3.PutObjectRetentionInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Retention: &s3types.ObjectLockRetention{
			Mode:            s3types.ObjectLockRetentionMode(mode),
			RetainUntilDate: aws.Time(retainTime),
		},
	}
	if awscommon.InputBool("bypass_governance_retention", inputs) {
		in.BypassGovernanceRetention = aws.Bool(true)
	}
	if versionID := awscommon.InputString("version_id", inputs); versionID != "" {
		in.VersionId = aws.String(versionID)
	}

	_, err = client.PutObjectRetention(ctx, in)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Set %s retention on %s/%s until %s", mode, bucket, key, retainTime.Format(time.RFC3339)),
		"bucket":       bucket,
		"key":          key,
		"mode":         mode,
		"retain_until": retainTime.Format(time.RFC3339),
	}, nil
}
