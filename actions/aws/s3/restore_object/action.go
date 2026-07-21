// Package aws_s3_restore_object initiates retrieval of an archived (Glacier)
// object.
package aws_s3_restore_object

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
	Name         = "AWS S3 Restore Object"
	Description  = "Initiate retrieval of an archived (Glacier) object for a number of days."
	Website      = "https://www.flomation.co"
	Icon         = "bucket+arrow-up"
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
	{Name: "key", Type: core.ConnectionTypeString, Label: "Object Key", Placeholder: "path/to/archive.zip", Required: true},
	{Name: "days", Type: core.ConnectionTypeInteger, Label: "Days to Retain Copy", Placeholder: "7", Required: true},
	{Name: "tier", Type: core.ConnectionTypeString, Label: "Retrieval Tier", Required: true, Options: []core.ConnectionOption{
		{Name: "Standard", Value: "Standard"},
		{Name: "Bulk", Value: "Bulk"},
		{Name: "Expedited", Value: "Expedited"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	bucket := awscommon.InputString("bucket", inputs)
	key := awscommon.InputString("key", inputs)
	if bucket == "" || key == "" {
		return nil, fmt.Errorf("bucket and key are required")
	}
	days, ok := awscommon.InputInt("days", inputs)
	if !ok || days <= 0 {
		return nil, fmt.Errorf("days must be a positive integer")
	}
	tier := awscommon.InputString("tier", inputs)
	if tier == "" {
		tier = "Standard"
	}

	restoreReq := &s3types.RestoreRequest{
		Days:                 aws.Int32(int32(days)),
		GlacierJobParameters: &s3types.GlacierJobParameters{Tier: s3types.Tier(tier)},
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := awsS3.NewFromConfig(cfg)

	_, err = client.RestoreObject(ctx, &awsS3.RestoreObjectInput{
		Bucket:         aws.String(bucket),
		Key:            aws.String(key),
		RestoreRequest: restoreReq,
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Requested %s restore of %s/%s for %d day(s)", tier, bucket, key, days),
	}, nil
}
