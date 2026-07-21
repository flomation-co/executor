// Package aws_s3_get_object_acl reads the access control list of an S3 object.
package aws_s3_get_object_acl

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
	Name         = "AWS S3 Get Object ACL"
	Description  = "Read the access control list (owner and grants) of an S3 object."
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "owner", Type: core.ConnectionTypeString, Label: "Owner"},
	{Name: "grants", Type: core.ConnectionTypeString, Label: "Grants (JSON)"},
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

	out, err := client.GetObjectAcl(ctx, &awsS3.GetObjectAclInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		return nil, err
	}

	var owner string
	if out.Owner != nil {
		owner = aws.ToString(out.Owner.DisplayName)
		if owner == "" {
			owner = aws.ToString(out.Owner.ID)
		}
	}

	type grantInfo struct {
		Grantee     string `json:"grantee"`
		GranteeType string `json:"grantee_type"`
		Permission  string `json:"permission"`
	}
	grants := make([]grantInfo, 0, len(out.Grants))
	for _, g := range out.Grants {
		gi := grantInfo{Permission: string(g.Permission)}
		if g.Grantee != nil {
			gi.GranteeType = string(g.Grantee.Type)
			switch {
			case g.Grantee.DisplayName != nil && *g.Grantee.DisplayName != "":
				gi.Grantee = *g.Grantee.DisplayName
			case g.Grantee.URI != nil && *g.Grantee.URI != "":
				gi.Grantee = *g.Grantee.URI
			default:
				gi.Grantee = aws.ToString(g.Grantee.ID)
			}
		}
		grants = append(grants, gi)
	}
	grantsJSON, err := json.Marshal(grants)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("%s/%s owned by %s with %d grant(s)", bucket, key, owner, len(grants)),
		"owner":       owner,
		"grants":      string(grantsJSON),
	}, nil
}
