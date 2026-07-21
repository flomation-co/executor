// Package aws_s3_upload_part uploads a single part of an S3 multipart upload.
package aws_s3_upload_part

import (
	"bytes"
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
	Name         = "AWS S3 Upload Part"
	Description  = "Upload one part of an S3 multipart upload and return its ETag."
	Website      = "https://www.flomation.co"
	Icon         = "layer-group+arrow-up"
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
	{Name: "key", Type: core.ConnectionTypeString, Label: "Object Key", Placeholder: "path/to/large-object", Required: true},
	{Name: "upload_id", Type: core.ConnectionTypeString, Label: "Upload ID", Required: true},
	{Name: "part_number", Type: core.ConnectionTypeInteger, Label: "Part Number", Placeholder: "1", Required: true},
	{Name: "content", Type: core.ConnectionTypeString, Label: "Part Contents", Placeholder: "Inline text, or a file/blob reference", Required: true},
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

	// The part body accepts a flo:file:/flo:blob: reference (e.g. a large media
	// action output) as well as inline text.
	content := awscommon.InputString("content", inputs)
	bodyBytes := []byte(content)
	if core.IsFileRef(content) || core.IsBlobToken(content) {
		resolved, _, rerr := flow.ResolveToBytes(content)
		if rerr != nil {
			return nil, rerr
		}
		bodyBytes = resolved
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := awsS3.NewFromConfig(cfg)

	out, err := client.UploadPart(ctx, &awsS3.UploadPartInput{
		Bucket:     aws.String(bucket),
		Key:        aws.String(key),
		UploadId:   aws.String(uploadID),
		PartNumber: aws.Int32(int32(partNumber)),
		Body:       bytes.NewReader(bodyBytes),
	})
	if err != nil {
		return nil, err
	}

	etag := aws.ToString(out.ETag)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Uploaded part %d (%d bytes) with ETag %s", partNumber, len(bodyBytes), etag),
		"etag":        etag,
		"part_number": partNumber,
	}, nil
}
