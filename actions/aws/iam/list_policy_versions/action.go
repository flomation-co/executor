// Package aws_iam_list_policy_versions lists the versions of an IAM managed policy.
package aws_iam_list_policy_versions

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
	Name         = "AWS IAM List Policy Versions"
	Description  = "List the versions of an IAM managed policy."
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
	{Name: "policy_arn", Type: core.ConnectionTypeString, Label: "Policy ARN", Placeholder: "arn:aws:iam::<account>:policy/MyPolicy", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "versions", Type: core.ConnectionTypeString, Label: "Versions (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

type versionSummary struct {
	VersionID        string `json:"version_id"`
	IsDefaultVersion bool   `json:"is_default_version"`
	CreateDate       string `json:"create_date"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	policyArn := awscommon.InputString("policy_arn", inputs)
	if policyArn == "" {
		return nil, fmt.Errorf("policy ARN is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := iam.NewFromConfig(cfg)

	in := &iam.ListPolicyVersionsInput{PolicyArn: aws.String(policyArn)}
	var summaries []versionSummary
	for {
		out, err := client.ListPolicyVersions(ctx, in)
		if err != nil {
			return nil, err
		}
		for _, v := range out.Versions {
			created := ""
			if v.CreateDate != nil {
				created = v.CreateDate.Format("2006-01-02T15:04:05Z07:00")
			}
			summaries = append(summaries, versionSummary{
				VersionID:        aws.ToString(v.VersionId),
				IsDefaultVersion: v.IsDefaultVersion,
				CreateDate:       created,
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

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Policy %s has %d version(s)", policyArn, len(summaries)),
		"versions":    string(encoded),
		"count":       len(summaries),
	}, nil
}
