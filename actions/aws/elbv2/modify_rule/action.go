// Package aws_elbv2_modify_rule modifies a listener rule.
package aws_elbv2_modify_rule

import (
	"context"
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS ELBv2 Modify Rule"
	Description  = "Change a listener rule's conditions or actions. Both are JSON arrays."
	Website      = "https://www.flomation.co"
	Icon         = "list+pen"
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
	{Name: "rule_arn", Type: core.ConnectionTypeString, Label: "Rule ARN", Placeholder: "arn:aws:elasticloadbalancing:...:listener-rule/...", Required: true},
	{Name: "conditions", Type: core.ConnectionTypeString, Label: "Conditions (JSON array)", Placeholder: `Optional [{"Field":"path-pattern",...}]`},
	{Name: "actions", Type: core.ConnectionTypeString, Label: "Actions (JSON array)", Placeholder: `Optional [{"Type":"forward","TargetGroupArn":"arn:..."}]`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "rule_arn", Type: core.ConnectionTypeString, Label: "Rule ARN"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	ruleARN := awscommon.InputString("rule_arn", inputs)
	if ruleARN == "" {
		return nil, fmt.Errorf("rule arn is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := elasticloadbalancingv2.NewFromConfig(cfg)

	in := &elasticloadbalancingv2.ModifyRuleInput{RuleArn: aws.String(ruleARN)}

	if raw := awscommon.InputString("conditions", inputs); raw != "" {
		var conditions []elbv2types.RuleCondition
		if err := json.Unmarshal([]byte(raw), &conditions); err != nil {
			return nil, fmt.Errorf("invalid conditions JSON: %w", err)
		}
		in.Conditions = conditions
	}
	if raw := awscommon.InputString("actions", inputs); raw != "" {
		var actions []elbv2types.Action
		if err := json.Unmarshal([]byte(raw), &actions); err != nil {
			return nil, fmt.Errorf("invalid actions JSON: %w", err)
		}
		in.Actions = actions
	}

	_, err = client.ModifyRule(ctx, in)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Modified rule %s", ruleARN),
		"rule_arn":    ruleARN,
	}, nil
}
