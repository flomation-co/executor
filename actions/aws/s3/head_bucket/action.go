// Package aws_s3_head_bucket checks whether an AWS S3 bucket exists/is accessible.
package aws_s3_head_bucket

import (
	"context"
	"errors"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsS3 "github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS S3 Head Bucket"
	Description  = "Check whether an AWS S3 bucket exists and is accessible."
	Website      = "https://www.flomation.co"
	Icon         = "bucket+magnifying-glass"
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
	{Name: "exists", Type: core.ConnectionTypeBoolean, Label: "Exists"},
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

	_, err = client.HeadBucket(ctx, &awsS3.HeadBucketInput{Bucket: aws.String(bucket)})
	if err != nil {
		// A missing/inaccessible bucket is not an execution failure — report
		// exists:false. Distinguish "not found" from genuine errors (auth, network).
		var notFound *s3types.NotFound
		var noSuchBucket *s3types.NoSuchBucket
		if errors.As(err, &notFound) || errors.As(err, &noSuchBucket) {
			return notExists(bucket), nil
		}
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			switch apiErr.ErrorCode() {
			case "NotFound", "NoSuchBucket", "404":
				return notExists(bucket), nil
			}
		}
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Bucket %s exists and is accessible", bucket),
		"exists":      true,
	}, nil
}

func notExists(bucket string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Bucket %s does not exist or is not accessible", bucket),
		"exists":      false,
	}
}
