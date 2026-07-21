// Package aws_s3_put_object_acl applies a canned access control list to an S3
// object.
package aws_s3_put_object_acl

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
	Name         = "AWS S3 Put Object ACL"
	Description  = "Apply a canned access control list (e.g. private, public-read) to an S3 object."
	Website      = "https://www.flomation.co"
	Icon         = "file-lines+shield-halved"
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
	{Name: "key", Type: core.ConnectionTypeString, Label: "Object Key", Placeholder: "path/to/object.txt", Required: true},
	{Name: "acl", Type: core.ConnectionTypeString, Label: "Canned ACL", Required: true, Options: []core.ConnectionOption{
		{Name: "Private", Value: "private"},
		{Name: "Public Read", Value: "public-read"},
		{Name: "Public Read/Write", Value: "public-read-write"},
		{Name: "Authenticated Read", Value: "authenticated-read"},
		{Name: "AWS Exec Read", Value: "aws-exec-read"},
		{Name: "Bucket Owner Read", Value: "bucket-owner-read"},
		{Name: "Bucket Owner Full Control", Value: "bucket-owner-full-control"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	bucket := awscommon.InputString("bucket", inputs)
	key := awscommon.InputString("key", inputs)
	acl := awscommon.InputString("acl", inputs)
	if bucket == "" || key == "" {
		return nil, fmt.Errorf("bucket and key are required")
	}
	if acl == "" {
		return nil, fmt.Errorf("a canned ACL is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := awsS3.NewFromConfig(cfg)

	_, err = client.PutObjectAcl(ctx, &awsS3.PutObjectAclInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		ACL:    s3types.ObjectCannedACL(acl),
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Applied ACL %q to %s/%s", acl, bucket, key),
	}, nil
}
