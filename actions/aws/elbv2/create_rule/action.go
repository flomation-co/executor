// Package aws_elbv2_create_rule creates a listener rule.
package aws_elbv2_create_rule

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
	Name         = "AWS ELBv2 Create Rule"
	Description  = "Create a listener rule. Conditions and actions are JSON arrays."
	Website      = "https://www.flomation.co"
	Icon         = "list+plus"
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
	{Name: "listener_arn", Type: core.ConnectionTypeString, Label: "Listener ARN", Placeholder: "arn:aws:elasticloadbalancing:...:listener/...", Required: true},
	{Name: "priority", Type: core.ConnectionTypeInteger, Label: "Priority", Placeholder: "1", Required: true},
	{Name: "conditions", Type: core.ConnectionTypeString, Label: "Conditions (JSON array)", Placeholder: `[{"Field":"path-pattern","PathPatternConfig":{"Values":["/api/*"]}}]`, Required: true},
	{Name: "actions", Type: core.ConnectionTypeString, Label: "Actions (JSON array)", Placeholder: `[{"Type":"forward","TargetGroupArn":"arn:..."}]`, Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "rule_arn", Type: core.ConnectionTypeString, Label: "Rule ARN"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	listenerARN := awscommon.InputString("listener_arn", inputs)
	if listenerARN == "" {
		return nil, fmt.Errorf("listener arn is required")
	}
	priority, ok := awscommon.InputInt("priority", inputs)
	if !ok {
		return nil, fmt.Errorf("priority is required")
	}
	conditionsRaw := awscommon.InputString("conditions", inputs)
	if conditionsRaw == "" {
		return nil, fmt.Errorf("conditions are required")
	}
	actionsRaw := awscommon.InputString("actions", inputs)
	if actionsRaw == "" {
		return nil, fmt.Errorf("actions are required")
	}

	var conditions []elbv2types.RuleCondition
	if err := json.Unmarshal([]byte(conditionsRaw), &conditions); err != nil {
		return nil, fmt.Errorf("invalid conditions JSON: %w", err)
	}
	var actions []elbv2types.Action
	if err := json.Unmarshal([]byte(actionsRaw), &actions); err != nil {
		return nil, fmt.Errorf("invalid actions JSON: %w", err)
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := elasticloadbalancingv2.NewFromConfig(cfg)

	out, err := client.CreateRule(ctx, &elasticloadbalancingv2.CreateRuleInput{
		ListenerArn: aws.String(listenerARN),
		Priority:    aws.Int32(int32(priority)),
		Conditions:  conditions,
		Actions:     actions,
	})
	if err != nil {
		return nil, err
	}
	if len(out.Rules) == 0 {
		return nil, fmt.Errorf("no rule returned")
	}

	ruleARN := aws.ToString(out.Rules[0].RuleArn)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created rule at priority %d: %s", priority, ruleARN),
		"rule_arn":    ruleARN,
	}, nil
}
