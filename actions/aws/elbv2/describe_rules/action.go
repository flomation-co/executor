// Package aws_elbv2_describe_rules lists listener rules.
package aws_elbv2_describe_rules

import (
	"context"
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS ELBv2 Describe Rules"
	Description  = "List listener rules by listener ARN or rule ARNs, with priority."
	Website      = "https://www.flomation.co"
	Icon         = "list+magnifying-glass"
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
	{Name: "listener_arn", Type: core.ConnectionTypeString, Label: "Listener ARN", Placeholder: "Optional — list all rules on this listener"},
	{Name: "rule_arns", Type: core.ConnectionTypeString, Label: "Rule ARNs", Placeholder: "Optional, comma-separated"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "rules", Type: core.ConnectionTypeString, Label: "Rules (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := elasticloadbalancingv2.NewFromConfig(cfg)

	in := &elasticloadbalancingv2.DescribeRulesInput{}
	if l := awscommon.InputString("listener_arn", inputs); l != "" {
		in.ListenerArn = aws.String(l)
	}
	if arns := awscommon.InputStrings("rule_arns", inputs); len(arns) > 0 {
		in.RuleArns = arns
	}

	type ruleInfo struct {
		ARN       string `json:"arn"`
		Priority  string `json:"priority"`
		IsDefault bool   `json:"is_default"`
	}
	var rules []ruleInfo

	paginator := elasticloadbalancingv2.NewDescribeRulesPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, r := range page.Rules {
			rules = append(rules, ruleInfo{
				ARN:       aws.ToString(r.RuleArn),
				Priority:  aws.ToString(r.Priority),
				IsDefault: aws.ToBool(r.IsDefault),
			})
		}
	}

	rulesJSON, err := json.Marshal(rules)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d rule(s)", len(rules)),
		"rules":       string(rulesJSON),
		"count":       len(rules),
	}, nil
}
