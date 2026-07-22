// Package aws_iam_list_policies lists IAM managed policies.
package aws_iam_list_policies

import (
	"context"
	"encoding/json"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS IAM List Policies"
	Description  = "List IAM managed policies, filtered by scope, path and attachment."
	Website      = "https://www.flomation.co"
	Icon         = "file-lines+list"
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
	{Name: "scope", Type: core.ConnectionTypeString, Label: "Scope", Options: []core.ConnectionOption{
		{Name: "All", Value: "All"},
		{Name: "AWS", Value: "AWS"},
		{Name: "Local", Value: "Local"},
	}},
	{Name: "path_prefix", Type: core.ConnectionTypeString, Label: "Path Prefix (optional)", Placeholder: "/division_abc/"},
	{Name: "only_attached", Type: core.ConnectionTypeBoolean, Label: "Only Attached Policies"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "policies", Type: core.ConnectionTypeString, Label: "Policies (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

type policySummary struct {
	PolicyName      string `json:"policy_name"`
	PolicyArn       string `json:"policy_arn"`
	AttachmentCount int32  `json:"attachment_count"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := iam.NewFromConfig(cfg)

	scope := strings.TrimSpace(awscommon.InputString("scope", inputs))
	if scope == "" {
		scope = "Local"
	}
	in := &iam.ListPoliciesInput{
		Scope:        iamtypes.PolicyScopeType(scope),
		OnlyAttached: awscommon.InputBool("only_attached", inputs),
	}
	if p := strings.TrimSpace(awscommon.InputString("path_prefix", inputs)); p != "" {
		in.PathPrefix = aws.String(p)
	}

	var summaries []policySummary
	for {
		out, err := client.ListPolicies(ctx, in)
		if err != nil {
			return nil, err
		}
		for _, p := range out.Policies {
			summaries = append(summaries, policySummary{
				PolicyName:      aws.ToString(p.PolicyName),
				PolicyArn:       aws.ToString(p.Arn),
				AttachmentCount: aws.ToInt32(p.AttachmentCount),
			})
		}
		if !out.IsTruncated || out.Marker == nil {
			break
		}
		in.Marker = out.Marker
	}

	encoded, err := json.Marshal(summaries)
	if err != nil {
		return nil, err
	}

	summary := "No policies found"
	if len(summaries) > 0 {
		summary = "Found policies"
	}
	return map[string]interface{}{
		"tool_result": summary,
		"policies":    string(encoded),
		"count":       len(summaries),
	}, nil
}
