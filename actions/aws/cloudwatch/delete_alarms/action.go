// Package aws_cloudwatch_delete_alarms deletes one or more CloudWatch alarms.
package aws_cloudwatch_delete_alarms

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS CloudWatch Delete Alarms"
	Description  = "Delete one or more CloudWatch alarms by name."
	Website      = "https://www.flomation.co"
	Icon         = "bell+trash"
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
	{Name: "alarm_names", Type: core.ConnectionTypeString, Label: "Alarm Names (comma-separated)", Placeholder: "high-cpu,high-mem", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "deleted_count", Type: core.ConnectionTypeInteger, Label: "Alarms Deleted"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	names := splitCSV(awscommon.InputString("alarm_names", inputs))
	if len(names) == 0 {
		return nil, fmt.Errorf("alarm_names is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := cloudwatch.NewFromConfig(cfg)

	if _, err := client.DeleteAlarms(ctx, &cloudwatch.DeleteAlarmsInput{AlarmNames: names}); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result":   fmt.Sprintf("Deleted %d alarm(s)", len(names)),
		"deleted_count": len(names),
	}, nil
}

func splitCSV(raw string) []string {
	var out []string
	for _, s := range strings.Split(raw, ",") {
		if t := strings.TrimSpace(s); t != "" {
			out = append(out, t)
		}
	}
	return out
}
