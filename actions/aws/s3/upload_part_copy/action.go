// Package aws_s3_upload_part_copy uploads a part of a multipart upload by
// copying data from an existing S3 object.
package aws_s3_upload_part_copy

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
	Name         = "AWS S3 Upload Part Copy"
	Description  = "Upload a multipart part by copying from an existing S3 object."
	Website      = "https://www.flomation.co"
	Icon         = "layer-group+copy"
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
	{Name: "key", Type: core.ConnectionTypeString, Label: "Destination Object Key", Placeholder: "path/to/large-object", Required: true},
	{Name: "upload_id", Type: core.ConnectionTypeString, Label: "Upload ID", Required: true},
	{Name: "part_number", Type: core.ConnectionTypeInteger, Label: "Part Number", Placeholder: "1", Required: true},
	{Name: "copy_source", Type: core.ConnectionTypeString, Label: "Copy Source", Placeholder: "source-bucket/source-key", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "etag", Type: core.ConnectionTypeString, Label: "ETag"},
	{Name: "part_number", Type: core.ConnectionTypeInteger, Label: "Part Number"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	bucket := awscommon.InputString("bucket", inputs)
	if bucket == "" {
		return nil, fmt.Errorf("bucket is required")
	}
	key := awscommon.InputString("key", inputs)
	if key == "" {
		return nil, fmt.Errorf("key is required")
	}
	uploadID := awscommon.InputString("upload_id", inputs)
	if uploadID == "" {
		return nil, fmt.Errorf("upload_id is required")
	}
	partNumber, ok := awscommon.InputInt("part_number", inputs)
	if !ok {
		return nil, fmt.Errorf("part_number is required")
	}
	copySource := awscommon.InputString("copy_source", inputs)
	if copySource == "" {
		return nil, fmt.Errorf("copy_source is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := awsS3.NewFromConfig(cfg)

	out, err := client.UploadPartCopy(ctx, &awsS3.UploadPartCopyInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		UploadId:   aws.String(uploadID),
		PartNumber: aws.Int32(int32(partNumber)),
		CopySource: aws.String(copySource),
	})
	if err != nil {
		return nil, err
	}

	var etag string
	if out.CopyPartResult != nil {
		etag = aws.ToString(out.CopyPartResult.ETag)
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Copied part %d from %s with ETag %s", partNumber, copySource, etag),
		"etag":        etag,
		"part_number": partNumber,
	}, nil
}
