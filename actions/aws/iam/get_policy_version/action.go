// Package aws_iam_get_policy_version retrieves a specific version of an IAM managed policy.
package aws_iam_get_policy_version

import (
	"context"
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS IAM Get Policy Version"
	Description  = "Retrieve a specific version of an IAM managed policy, including its document."
	Website      = "https://www.flomation.co"
	Icon         = "file-lines+magnifying-glass"
	Date         = "22/07/2026"
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
	{Name: "policy_arn", Type: core.ConnectionTypeString, Label: "Policy ARN", Placeholder: "arn:aws:iam::<account>:policy/MyPolicy", Required: true},
	{Name: "version_id", Type: core.ConnectionTypeString, Label: "Version ID", Placeholder: "v2", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "document", Type: core.ConnectionTypeString, Label: "Policy Document (JSON)"},
	{Name: "is_default_version", Type: core.ConnectionTypeBoolean, Label: "Is Default Version"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	policyArn := awscommon.InputString("policy_arn", inputs)
	if policyArn == "" {
		return nil, fmt.Errorf("policy ARN is required")
	}
	versionID := awscommon.InputString("version_id", inputs)
	if versionID == "" {
		return nil, fmt.Errorf("version ID is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := iam.NewFromConfig(cfg)

	out, err := client.GetPolicyVersion(ctx, &iam.GetPolicyVersionInput{
		PolicyArn: aws.String(policyArn),
		VersionId: aws.String(versionID),
	})
	if err != nil {
		return nil, err
	}

	var document string
	var isDefault bool
	if out.PolicyVersion != nil {
		raw := aws.ToString(out.PolicyVersion.Document)
		if decoded, derr := url.QueryUnescape(raw); derr == nil {
			document = decoded
		} else {
			document = raw
		}
		isDefault = out.PolicyVersion.IsDefaultVersion
	}

	return map[string]interface{}{
		"tool_result":        fmt.Sprintf("Retrieved policy version %s of %s (default=%t)", versionID, policyArn, isDefault),
		"document":           document,
		"is_default_version": isDefault,
	}, nil
}
