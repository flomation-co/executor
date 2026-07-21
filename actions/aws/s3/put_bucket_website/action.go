// Package aws_s3_put_bucket_website configures static-website hosting on an S3
// bucket.
package aws_s3_put_bucket_website

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
	Name         = "AWS S3 Put Bucket Website"
	Description  = "Configure static-website hosting on an S3 bucket (index and error documents)."
	Website      = "https://www.flomation.co"
	Icon         = "bucket+globe"
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
	{Name: "index_suffix", Type: core.ConnectionTypeString, Label: "Index Document", Placeholder: "index.html", Required: true},
	{Name: "error_key", Type: core.ConnectionTypeString, Label: "Error Document (optional)", Placeholder: "error.html"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	bucket := awscommon.InputString("bucket", inputs)
	if bucket == "" {
		return nil, fmt.Errorf("bucket is required")
	}
	indexSuffix := awscommon.InputString("index_suffix", inputs)
	if indexSuffix == "" {
		return nil, fmt.Errorf("index document suffix is required")
	}

	websiteCfg := &s3types.WebsiteConfiguration{
		IndexDocument: &s3types.IndexDocument{Suffix: aws.String(indexSuffix)},
	}
	if errorKey := awscommon.InputString("error_key", inputs); errorKey != "" {
		websiteCfg.ErrorDocument = &s3types.ErrorDocument{Key: aws.String(errorKey)}
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := awsS3.NewFromConfig(cfg)

	_, err = client.PutBucketWebsite(ctx, &awsS3.PutBucketWebsiteInput{
		Bucket:               aws.String(bucket),
		WebsiteConfiguration: websiteCfg,
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Configured website hosting on %s (index %s)", bucket, indexSuffix),
	}, nil
}
