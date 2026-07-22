// Package aws_cloudwatch_describe_alarms lists CloudWatch metric alarms with
// their current state.
package aws_cloudwatch_describe_alarms

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS CloudWatch Describe Alarms"
	Description  = "List metric alarms with their current state and thresholds."
	Website      = "https://www.flomation.co"
	Icon         = "bell+magnifying-glass"
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
	{Name: "alarm_names", Type: core.ConnectionTypeString, Label: "Alarm Names (comma-separated, optional)", Placeholder: "high-cpu,high-mem"},
	{Name: "state_value", Type: core.ConnectionTypeString, Label: "State (optional)", Options: []core.ConnectionOption{
		{Name: "OK", Value: "OK"},
		{Name: "ALARM", Value: "ALARM"},
		{Name: "INSUFFICIENT_DATA", Value: "INSUFFICIENT_DATA"},
	}},
	{Name: "alarm_name_prefix", Type: core.ConnectionTypeString, Label: "Alarm Name Prefix (optional)", Placeholder: "prod-"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "alarms", Type: core.ConnectionTypeString, Label: "Alarms (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Alarm Count"},
}

type alarmSummary struct {
	Name        string   `json:"name"`
	StateValue  string   `json:"state_value"`
	StateReason string   `json:"state_reason"`
	MetricName  string   `json:"metric_name"`
	Threshold   *float64 `json:"threshold"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := cloudwatch.NewFromConfig(cfg)

	in := &cloudwatch.DescribeAlarmsInput{}
	if names := splitCSV(awscommon.InputString("alarm_names", inputs)); len(names) > 0 {
		in.AlarmNames = names
	}
	if sv := strings.TrimSpace(awscommon.InputString("state_value", inputs)); sv != "" {
		in.StateValue = cwtypes.StateValue(sv)
	}
	if prefix := strings.TrimSpace(awscommon.InputString("alarm_name_prefix", inputs)); prefix != "" {
		in.AlarmNamePrefix = aws.String(prefix)
	}

	var summaries []alarmSummary
	paginator := cloudwatch.NewDescribeAlarmsPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, a := range page.MetricAlarms {
			summaries = append(summaries, alarmSummary{
				Name:        aws.ToString(a.AlarmName),
				StateValue:  string(a.StateValue),
				StateReason: aws.ToString(a.StateReason),
				MetricName:  aws.ToString(a.MetricName),
				Threshold:   a.Threshold,
			})
		}
	}

	encoded, err := json.Marshal(summaries)
	if err != nil {
		return nil, fmt.Errorf("encode alarms: %w", err)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d alarm(s)", len(summaries)),
		"alarms":      string(encoded),
		"count":       len(summaries),
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
