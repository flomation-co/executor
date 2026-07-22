// Package aws_iam_list_attached_group_policies lists managed policies attached to an IAM group.
package aws_iam_list_attached_group_policies

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
	Name         = "AWS IAM List Attached Group Policies"
	Description  = "List the managed policies attached to an IAM group."
	Website      = "https://www.flomation.co"
	Icon         = "user-group+list"
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
	{Name: "group_name", Type: core.ConnectionTypeString, Label: "Group Name", Placeholder: "Developers", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "attached_policies", Type: core.ConnectionTypeString, Label: "Attached Policies (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

type attachedPolicy struct {
	PolicyName string `json:"policy_name"`
	PolicyArn  string `json:"policy_arn"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	groupName := awscommon.InputString("group_name", inputs)
	if groupName == "" {
		return nil, fmt.Errorf("group name is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := iam.NewFromConfig(cfg)

	in := &iam.ListAttachedGroupPoliciesInput{GroupName: aws.String(groupName)}
	var policies []attachedPolicy
	for {
		out, err := client.ListAttachedGroupPolicies(ctx, in)
		if err != nil {
			return nil, err
		}
		for _, p := range out.AttachedPolicies {
			policies = append(policies, attachedPolicy{
				PolicyName: aws.ToString(p.PolicyName),
				PolicyArn:  aws.ToString(p.PolicyArn),
			})
		}
		if !out.IsTruncated || out.Marker == nil {
			break
		}
		in.Marker = out.Marker
	}

	encoded, err := json.Marshal(policies)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result":       fmt.Sprintf("Group %s has %d attached policy(ies)", groupName, len(policies)),
		"attached_policies": string(encoded),
		"count":             len(policies),
	}, nil
}
