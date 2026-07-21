// Package aws_elbv2_register_targets registers targets with an ELBv2 target group.
package aws_elbv2_register_targets

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Register Targets"
	Description  = "Register targets with an Elastic Load Balancing target group."
	Website      = "https://www.flomation.co"
	Icon         = "diagram-project+link"
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
	{Name: "target_group_arn", Type: core.ConnectionTypeString, Label: "Target Group ARN", Placeholder: "arn:aws:elasticloadbalancing:...:targetgroup/...", Required: true},
	{Name: "targets", Type: core.ConnectionTypeString, Label: "Targets", Placeholder: `[{"id":"i-0abc123","port":80}] or i-0abc123,i-0def456`, Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "target_count", Type: core.ConnectionTypeInteger, Label: "Target Count"},
}

// parseTargets accepts either a JSON array of {id, port?} objects or a
// comma-separated list of target ids.
func parseTargets(raw string) ([]elbv2types.TargetDescription, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("at least one target is required")
	}
	var out []elbv2types.TargetDescription
	if strings.HasPrefix(raw, "[") {
		var entries []struct {
			ID   string `json:"id"`
			Port *int32 `json:"port"`
		}
		if err := json.Unmarshal([]byte(raw), &entries); err != nil {
			return nil, fmt.Errorf("invalid targets JSON: %w", err)
		}
		for _, e := range entries {
			id := strings.TrimSpace(e.ID)
			if id == "" {
				continue
			}
			td := elbv2types.TargetDescription{Id: aws.String(id)}
			if e.Port != nil {
				td.Port = e.Port
			}
			out = append(out, td)
		}
	} else {
		for _, id := range strings.Split(raw, ",") {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			out = append(out, elbv2types.TargetDescription{Id: aws.String(id)})
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("at least one target is required")
	}
	return out, nil
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	arn := awscommon.InputString("target_group_arn", inputs)
	if arn == "" {
		return nil, fmt.Errorf("target group arn is required")
	}
	targets, err := parseTargets(awscommon.InputString("targets", inputs))
	if err != nil {
		return nil, err
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := elasticloadbalancingv2.NewFromConfig(cfg)

	_, err = client.RegisterTargets(ctx, &elasticloadbalancingv2.RegisterTargetsInput{
		TargetGroupArn: aws.String(arn),
		Targets:        targets,
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Registered %d target(s) with %s", len(targets), arn),
		"target_count": len(targets),
	}, nil
}
