// Package aws_iam_list_role_policies lists inline policy names on an IAM role.
package aws_iam_list_role_policies

import (
	"context"
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS IAM List Role Policies"
	Description  = "List the inline policy names attached to an IAM role."
	Website      = "https://www.flomation.co"
	Icon         = "shield-halved+list"
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
	{Name: "role_name", Type: core.ConnectionTypeString, Label: "Role Name", Placeholder: "my-role", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "policy_names", Type: core.ConnectionTypeString, Label: "Policy Names (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	roleName := awscommon.InputString("role_name", inputs)
	if roleName == "" {
		return nil, fmt.Errorf("role name is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := iam.NewFromConfig(cfg)

	var names []string
	paginator := iam.NewListRolePoliciesPaginator(client, &iam.ListRolePoliciesInput{
		RoleName: aws.String(roleName),
	})
	for paginator.HasMorePages() {
		page, perr := paginator.NextPage(ctx)
		if perr != nil {
			return nil, perr
		}
		names = append(names, page.PolicyNames...)
	}

	namesJSON, err := json.Marshal(names)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Found %d inline policies on role %s", len(names), roleName),
		"policy_names": string(namesJSON),
		"count":        len(names),
	}, nil
}
