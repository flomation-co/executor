// Package aws_s3_list_object_versions lists the versions of objects in a
// versioning-enabled bucket.
package aws_s3_list_object_versions

import (
	"context"
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsS3 "github.com/aws/aws-sdk-go-v2/service/s3"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS S3 List Object Versions"
	Description  = "List object versions in a versioning-enabled S3 bucket, optionally by prefix."
	Website      = "https://www.flomation.co"
	Icon         = "file-lines+clock"
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
	{Name: "prefix", Type: core.ConnectionTypeString, Label: "Prefix (optional)", Placeholder: "path/to/"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "versions", Type: core.ConnectionTypeString, Label: "Versions (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Version Count"},
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

	in := &awsS3.ListObjectVersionsInput{Bucket: aws.String(bucket)}
	if prefix := awscommon.InputString("prefix", inputs); prefix != "" {
		in.Prefix = aws.String(prefix)
	}

	out, err := client.ListObjectVersions(ctx, in)
	if err != nil {
		return nil, err
	}

	type versionInfo struct {
		Key          string `json:"key"`
		VersionID    string `json:"version_id"`
		IsLatest     bool   `json:"is_latest"`
		Size         int64  `json:"size"`
		LastModified string `json:"last_modified"`
	}
	versions := make([]versionInfo, 0, len(out.Versions))
	for _, v := range out.Versions {
		var lastModified string
		if v.LastModified != nil {
			lastModified = v.LastModified.UTC().Format("2006-01-02T15:04:05Z07:00")
		}
		versions = append(versions, versionInfo{
			Key:          aws.ToString(v.Key),
			VersionID:    aws.ToString(v.VersionId),
			IsLatest:     aws.ToBool(v.IsLatest),
			Size:         aws.ToInt64(v.Size),
			LastModified: lastModified,
		})
	}
	versionsJSON, err := json.Marshal(versions)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d object version(s) in %s", len(versions), bucket),
		"versions":    string(versionsJSON),
		"count":       len(versions),
	}, nil
}
