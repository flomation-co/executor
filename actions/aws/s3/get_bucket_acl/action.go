// Package aws_s3_get_bucket_acl reads a bucket's access control list.
package aws_s3_get_bucket_acl

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
	Name         = "AWS S3 Get Bucket ACL"
	Description  = "Read the access control list (owner and grants) of an AWS S3 bucket."
	Website      = "https://www.flomation.co"
	Icon         = "bucket+shield-halved"
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
	{Name: "owner", Type: core.ConnectionTypeString, Label: "Owner"},
	{Name: "grants", Type: core.ConnectionTypeString, Label: "Grants (JSON)"},
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

	out, err := client.GetBucketAcl(ctx, &awsS3.GetBucketAclInput{Bucket: aws.String(bucket)})
	if err != nil {
		return nil, err
	}

	var owner string
	if out.Owner != nil {
		owner = aws.ToString(out.Owner.ID)
		if dn := aws.ToString(out.Owner.DisplayName); dn != "" {
			owner = dn
		}
	}

	type grantOut struct {
		GranteeType string `json:"grantee_type"`
		Grantee     string `json:"grantee"`
		Permission  string `json:"permission"`
	}
	grants := make([]grantOut, 0, len(out.Grants))
	for _, g := range out.Grants {
		go2 := grantOut{Permission: string(g.Permission)}
		if g.Grantee != nil {
			go2.GranteeType = string(g.Grantee.Type)
			switch {
			case aws.ToString(g.Grantee.ID) != "":
				go2.Grantee = aws.ToString(g.Grantee.ID)
			case aws.ToString(g.Grantee.URI) != "":
				go2.Grantee = aws.ToString(g.Grantee.URI)
			case aws.ToString(g.Grantee.EmailAddress) != "":
				go2.Grantee = aws.ToString(g.Grantee.EmailAddress)
			default:
				go2.Grantee = aws.ToString(g.Grantee.DisplayName)
			}
		}
		grants = append(grants, go2)
	}

	grantsJSON, err := json.Marshal(grants)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Bucket %s has %d grant(s), owner %s", bucket, len(grants), owner),
		"owner":       owner,
		"grants":      string(grantsJSON),
	}, nil
}
