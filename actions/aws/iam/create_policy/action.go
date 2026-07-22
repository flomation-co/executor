// Package aws_iam_create_policy creates an IAM managed policy.
package aws_iam_create_policy

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS IAM Create Policy"
	Description  = "Create an IAM managed policy from a JSON document."
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
	{Name: "policy_name", Type: core.ConnectionTypeString, Label: "Policy Name", Placeholder: "MyPolicy", Required: true},
	{Name: "policy_document", Type: core.ConnectionTypeString, Label: "Policy Document (JSON)", Placeholder: `{"Version":"2012-10-17","Statement":[...]}`, Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description (optional)"},
	{Name: "path", Type: core.ConnectionTypeString, Label: "Path (optional)", Placeholder: "/division_abc/"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "policy_name", Type: core.ConnectionTypeString, Label: "Policy Name"},
	{Name: "policy_arn", Type: core.ConnectionTypeString, Label: "Policy ARN"},
	{Name: "policy_id", Type: core.ConnectionTypeString, Label: "Policy ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	policyName := awscommon.InputString("policy_name", inputs)
	if policyName == "" {
		return nil, fmt.Errorf("policy name is required")
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

	in := &iam.CreatePolicyInput{
		PolicyName:     aws.String(policyName),
		PolicyDocument: aws.String(policyDocument),
	}
	if d := strings.TrimSpace(awscommon.InputString("description", inputs)); d != "" {
		in.Description = aws.String(d)
	}
	if p := strings.TrimSpace(awscommon.InputString("path", inputs)); p != "" {
		in.Path = aws.String(p)
	}

	out, err := client.CreatePolicy(ctx, in)
	if err != nil {
		return nil, err
	}

	var policyArn, policyID string
	if out.Policy != nil {
		policyArn = aws.ToString(out.Policy.Arn)
		policyID = aws.ToString(out.Policy.PolicyId)
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created IAM policy %s (%s)", policyName, policyArn),
		"policy_name": policyName,
		"policy_arn":  policyArn,
		"policy_id":   policyID,
	}, nil
}
