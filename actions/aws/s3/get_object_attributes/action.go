// Package aws_s3_get_object_attributes retrieves an S3 object's attributes
// (ETag, storage class, size) without downloading the object body.
package aws_s3_get_object_attributes

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
	Name         = "AWS S3 Get Object Attributes"
	Description  = "Retrieve an S3 object's ETag, storage class and size without downloading the body."
	Website      = "https://www.flomation.co"
	Icon         = "file-lines+circle-info"
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
	{Name: "version_id", Type: core.ConnectionTypeString, Label: "Version ID (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "etag", Type: core.ConnectionTypeString, Label: "ETag"},
	{Name: "storage_class", Type: core.ConnectionTypeString, Label: "Storage Class"},
	{Name: "object_size", Type: core.ConnectionTypeInteger, Label: "Object Size"},
	{Name: "last_modified", Type: core.ConnectionTypeString, Label: "Last Modified"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	bucket := awscommon.InputString("bucket", inputs)
	key := awscommon.InputString("key", inputs)
	if bucket == "" || key == "" {
		return nil, fmt.Errorf("bucket and key are required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := awsS3.NewFromConfig(cfg)

	in := &awsS3.GetObjectAttributesInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		ObjectAttributes: []s3types.ObjectAttributes{
			s3types.ObjectAttributesEtag,
			s3types.ObjectAttributesStorageClass,
			s3types.ObjectAttributesObjectSize,
		},
	}
	if versionID := awscommon.InputString("version_id", inputs); versionID != "" {
		in.VersionId = aws.String(versionID)
	}

	out, err := client.GetObjectAttributes(ctx, in)
	if err != nil {
		return nil, err
	}

	var lastModified string
	if out.LastModified != nil {
		lastModified = out.LastModified.UTC().Format("2006-01-02T15:04:05Z07:00")
	}
	storageClass := string(out.StorageClass)

	return map[string]interface{}{
		"tool_result":   fmt.Sprintf("%s/%s: %d bytes, storage class %s", bucket, key, aws.ToInt64(out.ObjectSize), storageClass),
		"etag":          aws.ToString(out.ETag),
		"storage_class": storageClass,
		"object_size":   aws.ToInt64(out.ObjectSize),
		"last_modified": lastModified,
	}, nil
}
