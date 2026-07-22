// Package aws_iam_simulate_custom_policy simulates IAM evaluation of supplied policy documents.
package aws_iam_simulate_custom_policy

import (
	"context"
	"encoding/json"
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
	Name         = "AWS IAM Simulate Custom Policy"
	Description  = "Simulate how supplied IAM policy documents evaluate against actions and resources."
	Website      = "https://www.flomation.co"
	Icon         = "file-lines+circle-check"
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
	{Name: "policy_input_list", Type: core.ConnectionTypeString, Label: "Policy Documents (JSON array)", Placeholder: `["{\"Version\":\"2012-10-17\",\"Statement\":[...]}"]`, Required: true},
	{Name: "action_names", Type: core.ConnectionTypeString, Label: "Action Names (comma-separated)", Placeholder: "s3:GetObject, s3:PutObject", Required: true},
	{Name: "resource_arns", Type: core.ConnectionTypeString, Label: "Resource ARNs (comma-separated, optional)", Placeholder: "arn:aws:s3:::my-bucket/*"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "evaluation_results", Type: core.ConnectionTypeString, Label: "Evaluation Results (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

type evaluationResult struct {
	Action   string `json:"action"`
	Decision string `json:"decision"`
	Resource string `json:"resource"`
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	rawPolicies := strings.TrimSpace(awscommon.InputString("policy_input_list", inputs))
	if rawPolicies == "" {
		return nil, fmt.Errorf("policy input list is required")
	}
	var policyInputList []string
	if err := json.Unmarshal([]byte(rawPolicies), &policyInputList); err != nil {
		return nil, fmt.Errorf("policy input list must be a JSON array of policy documents: %w", err)
	}
	if len(policyInputList) == 0 {
		return nil, fmt.Errorf("at least one policy document is required")
	}

	actionNames := splitList(awscommon.InputString("action_names", inputs))
	if len(actionNames) == 0 {
		return nil, fmt.Errorf("at least one action name is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := iam.NewFromConfig(cfg)

	in := &iam.SimulateCustomPolicyInput{
		PolicyInputList: policyInputList,
		ActionNames:     actionNames,
	}
	if resources := splitList(awscommon.InputString("resource_arns", inputs)); len(resources) > 0 {
		in.ResourceArns = resources
	}

	out, err := client.SimulateCustomPolicy(ctx, in)
	if err != nil {
		return nil, err
	}

	results := []evaluationResult{}
	for _, r := range out.EvaluationResults {
		results = append(results, evaluationResult{
			Action:   aws.ToString(r.EvalActionName),
			Decision: string(r.EvalDecision),
			Resource: aws.ToString(r.EvalResourceName),
		})
	}

	resultsJSON, err := json.Marshal(results)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result":        fmt.Sprintf("Simulated %d action evaluations", len(results)),
		"evaluation_results": string(resultsJSON),
		"count":              len(results),
	}, nil
}
