// Package aws_s3_get_bucket_encryption reads a bucket's default encryption.
package aws_s3_get_bucket_encryption

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
	Name         = "AWS S3 Get Bucket Encryption"
	Description  = "Read the default server-side encryption of an AWS S3 bucket."
	Website      = "https://www.flomation.co"
	Icon         = "bucket+lock"
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
	{Name: "sse_algorithm", Type: core.ConnectionTypeString, Label: "Encryption Algorithm"},
	{Name: "kms_key_id", Type: core.ConnectionTypeString, Label: "KMS Key ID"},
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

	// GetBucketEncryption returns an error when no default encryption is set.
	out, err := client.GetBucketEncryption(ctx, &awsS3.GetBucketEncryptionInput{Bucket: aws.String(bucket)})
	if err != nil {
		return nil, err
	}

	var algorithm, kmsKeyID string
	if cfg := out.ServerSideEncryptionConfiguration; cfg != nil && len(cfg.Rules) > 0 {
		if def := cfg.Rules[0].ApplyServerSideEncryptionByDefault; def != nil {
			algorithm = string(def.SSEAlgorithm)
			kmsKeyID = aws.ToString(def.KMSMasterKeyID)
		}
	}

	return map[string]interface{}{
		"tool_result":   fmt.Sprintf("Bucket %s default encryption: %s", bucket, algorithm),
		"sse_algorithm": algorithm,
		"kms_key_id":    kmsKeyID,
	}, nil
}
