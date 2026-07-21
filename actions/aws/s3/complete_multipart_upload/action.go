// Package aws_s3_complete_multipart_upload completes an S3 multipart upload by
// assembling the previously uploaded parts.
package aws_s3_complete_multipart_upload

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsS3 "github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS S3 Complete Multipart Upload"
	Description  = "Complete an S3 multipart upload from a JSON list of part ETags."
	Website      = "https://www.flomation.co"
	Icon         = "layer-group+circle-check"
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
	{Name: "parts", Type: core.ConnectionTypeString, Label: "Parts (JSON array)", Placeholder: `[{"etag":"\"abc123\"","part_number":1}]`, Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "location", Type: core.ConnectionTypeString, Label: "Location"},
	{Name: "etag", Type: core.ConnectionTypeString, Label: "ETag"},
	{Name: "version_id", Type: core.ConnectionTypeString, Label: "Version ID"},
}

// completedPart is the curated JSON shape a caller supplies for each part.
type completedPart struct {
	ETag       string `json:"etag"`
	PartNumber int32  `json:"part_number"`
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
	partsRaw := strings.TrimSpace(awscommon.InputString("parts", inputs))
	if partsRaw == "" {
		return nil, fmt.Errorf("parts JSON array is required")
	}

	var parsed []completedPart
	if err := json.Unmarshal([]byte(partsRaw), &parsed); err != nil {
		return nil, fmt.Errorf("parts must be a JSON array: %w", err)
	}
	if len(parsed) == 0 {
		return nil, fmt.Errorf("at least one part is required")
	}

	parts := make([]s3types.CompletedPart, 0, len(parsed))
	for _, p := range parsed {
		parts = append(parts, s3types.CompletedPart{
			ETag:       aws.String(p.ETag),
			PartNumber: aws.Int32(p.PartNumber),
		})
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := awsS3.NewFromConfig(cfg)

	out, err := client.CompleteMultipartUpload(ctx, &awsS3.CompleteMultipartUploadInput{
		Bucket:          aws.String(bucket),
		Key:             aws.String(key),
		UploadId:        aws.String(uploadID),
		MultipartUpload: &s3types.CompletedMultipartUpload{Parts: parts},
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Completed multipart upload of %s/%s from %d part(s)", bucket, key, len(parts)),
		"location":    aws.ToString(out.Location),
		"etag":        aws.ToString(out.ETag),
		"version_id":  aws.ToString(out.VersionId),
	}, nil
}
