// Package aws_autoscaling_start_instance_refresh starts an instance refresh.
package aws_autoscaling_start_instance_refresh

import (
	"context"
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	astypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Auto Scaling Start Instance Refresh"
	Description  = "Start a rolling instance refresh on an Auto Scaling group (preferences are JSON)."
	Website      = "https://www.flomation.co"
	Icon         = "arrows-up-down+arrow-up"
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
	{Name: "auto_scaling_group_name", Type: core.ConnectionTypeString, Label: "Auto Scaling Group Name", Placeholder: "my-asg", Required: true},
	{Name: "strategy", Type: core.ConnectionTypeString, Label: "Strategy", Placeholder: "Rolling", Options: []core.ConnectionOption{
		{Name: "Rolling", Value: "Rolling"},
	}},
	{Name: "preferences", Type: core.ConnectionTypeString, Label: "Preferences (JSON)", Placeholder: `{"MinHealthyPercentage":90,"InstanceWarmup":300}`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "instance_refresh_id", Type: core.ConnectionTypeString, Label: "Instance Refresh ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	groupName := awscommon.InputString("auto_scaling_group_name", inputs)
	if groupName == "" {
		return nil, fmt.Errorf("auto scaling group name is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := autoscaling.NewFromConfig(cfg)

	strategy := awscommon.InputString("strategy", inputs)
	if strategy == "" {
		strategy = "Rolling"
	}

	in := &autoscaling.StartInstanceRefreshInput{
		AutoScalingGroupName: aws.String(groupName),
		Strategy:             astypes.RefreshStrategy(strategy),
	}
	if raw := awscommon.InputString("preferences", inputs); raw != "" {
		var prefs astypes.RefreshPreferences
		if err := json.Unmarshal([]byte(raw), &prefs); err != nil {
			return nil, fmt.Errorf("invalid preferences JSON: %w", err)
		}
		in.Preferences = &prefs
	}

	out, err := client.StartInstanceRefresh(ctx, in)
	if err != nil {
		return nil, err
	}

	refreshID := aws.ToString(out.InstanceRefreshId)
	return map[string]interface{}{
		"tool_result":         fmt.Sprintf("Started instance refresh %s on %s", refreshID, groupName),
		"instance_refresh_id": refreshID,
	}, nil
}
