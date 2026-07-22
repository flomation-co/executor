// Package aws_iam_create_policy_version adds a new version to an IAM managed policy.
package aws_iam_create_policy_version

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS IAM Create Policy Version"
	Description  = "Add a new version to an IAM managed policy, optionally as default."
	Website      = "https://www.flomation.co"
	Icon         = "file-lines+plus"
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
	{Name: "policy_document", Type: core.ConnectionTypeString, Label: "Policy Document (JSON)", Placeholder: `{"Version":"2012-10-17","Statement":[...]}`, Required: true},
	{Name: "set_as_default", Type: core.ConnectionTypeBoolean, Label: "Set As Default Version"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "version_id", Type: core.ConnectionTypeString, Label: "Version ID"},
	{Name: "is_default", Type: core.ConnectionTypeBoolean, Label: "Is Default"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	policyArn := awscommon.InputString("policy_arn", inputs)
	if policyArn == "" {
		return nil, fmt.Errorf("policy ARN is required")
	}
	policyDocument := awscommon.InputString("policy_document", inputs)
	if policyDocument == "" {
		return nil, fmt.Errorf("policy document is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := iam.NewFromConfig(cfg)

	out, err := client.CreatePolicyVersion(ctx, &iam.CreatePolicyVersionInput{
		PolicyArn:      aws.String(policyArn),
		PolicyDocument: aws.String(policyDocument),
		SetAsDefault:   awscommon.InputBool("set_as_default", inputs),
	})
	if err != nil {
		return nil, err
	}

	var versionID string
	var isDefault bool
	if out.PolicyVersion != nil {
		versionID = aws.ToString(out.PolicyVersion.VersionId)
		isDefault = out.PolicyVersion.IsDefaultVersion
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created policy version %s (default=%t)", versionID, isDefault),
		"version_id":  versionID,
		"is_default":  isDefault,
	}, nil
}
