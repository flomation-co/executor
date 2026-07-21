// Package aws_s3_copy_object copies an object within or between S3 buckets.
package aws_s3_copy_object

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
	Name         = "AWS S3 Copy Object"
	Description  = "Copy an object to a destination bucket and key within AWS S3."
	Website      = "https://www.flomation.co"
	Icon         = "file-lines+copy"
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
	{Name: "bucket", Type: core.ConnectionTypeString, Label: "Destination Bucket", Placeholder: "my-bucket", Required: true},
	{Name: "key", Type: core.ConnectionTypeString, Label: "Destination Key", Placeholder: "path/to/destination.txt", Required: true},
	{Name: "copy_source", Type: core.ConnectionTypeString, Label: "Copy Source", Placeholder: "source-bucket/source-key.txt", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "etag", Type: core.ConnectionTypeString, Label: "ETag"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	bucket := awscommon.InputString("bucket", inputs)
	key := awscommon.InputString("key", inputs)
	copySource := awscommon.InputString("copy_source", inputs)
	if bucket == "" || key == "" {
		return nil, fmt.Errorf("destination bucket and key are required")
	}
	if copySource == "" {
		return nil, fmt.Errorf("copy_source (source-bucket/source-key) is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := awsS3.NewFromConfig(cfg)

	out, err := client.CopyObject(ctx, &awsS3.CopyObjectInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		CopySource: aws.String(copySource),
	})
	if err != nil {
		return nil, err
	}

	var etag string
	if out.CopyObjectResult != nil {
		etag = aws.ToString(out.CopyObjectResult.ETag)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Copied %s to %s/%s", copySource, bucket, key),
		"etag":        etag,
	}, nil
}
