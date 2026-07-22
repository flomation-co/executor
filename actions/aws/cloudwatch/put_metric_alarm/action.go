// Package aws_cloudwatch_put_metric_alarm creates or updates a CloudWatch
// metric alarm.
package aws_cloudwatch_put_metric_alarm

import (
	"context"
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
	Name         = "AWS CloudWatch Put Metric Alarm"
	Description  = "Create or update a metric alarm on a threshold comparison."
	Website      = "https://www.flomation.co"
	Icon         = "bell+plus"
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
	{Name: "alarm_name", Type: core.ConnectionTypeString, Label: "Alarm Name", Placeholder: "high-cpu", Required: true},
	{Name: "comparison_operator", Type: core.ConnectionTypeString, Label: "Comparison Operator", Required: true, Options: []core.ConnectionOption{
		{Name: "Greater Than Threshold", Value: "GreaterThanThreshold"},
		{Name: "Greater Than Or Equal To Threshold", Value: "GreaterThanOrEqualToThreshold"},
		{Name: "Less Than Threshold", Value: "LessThanThreshold"},
		{Name: "Less Than Or Equal To Threshold", Value: "LessThanOrEqualToThreshold"},
	}},
	{Name: "evaluation_periods", Type: core.ConnectionTypeInteger, Label: "Evaluation Periods", Placeholder: "1", Required: true},
	{Name: "metric_name", Type: core.ConnectionTypeString, Label: "Metric Name", Placeholder: "CPUUtilization"},
	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "AWS/EC2"},
	{Name: "period", Type: core.ConnectionTypeInteger, Label: "Period (seconds)", Placeholder: "300"},
	{Name: "statistic", Type: core.ConnectionTypeString, Label: "Statistic", Options: []core.ConnectionOption{
		{Name: "Average", Value: "Average"},
		{Name: "Sum", Value: "Sum"},
		{Name: "Minimum", Value: "Minimum"},
		{Name: "Maximum", Value: "Maximum"},
		{Name: "Sample Count", Value: "SampleCount"},
	}},
	{Name: "threshold", Type: core.ConnectionTypeString, Label: "Threshold", Placeholder: "80", Required: true},
	{Name: "alarm_actions", Type: core.ConnectionTypeString, Label: "Alarm Actions (comma-separated ARNs)", Placeholder: "arn:aws:sns:eu-west-2:123456789012:alerts"},
	{Name: "dimensions", Type: core.ConnectionTypeKeyValueArray, Label: "Dimensions", Placeholder: "Add a Name and Value per dimension"},
	{Name: "alarm_description", Type: core.ConnectionTypeString, Label: "Description (optional)"},
	{Name: "treat_missing_data", Type: core.ConnectionTypeString, Label: "Treat Missing Data (optional)", Placeholder: "missing | notBreaching | breaching | ignore"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "alarm_name", Type: core.ConnectionTypeString, Label: "Alarm Name"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	alarmName := strings.TrimSpace(awscommon.InputString("alarm_name", inputs))
	if alarmName == "" {
		return nil, fmt.Errorf("alarm_name is required")
	}
	comparison := strings.TrimSpace(awscommon.InputString("comparison_operator", inputs))
	if comparison == "" {
		return nil, fmt.Errorf("comparison_operator is required")
	}
	evalPeriods, ok := awscommon.InputInt("evaluation_periods", inputs)
	if !ok || evalPeriods <= 0 {
		return nil, fmt.Errorf("evaluation_periods is required and must be positive")
	}
	threshold, ok := awscommon.InputFloat("threshold", inputs)
	if !ok {
		return nil, fmt.Errorf("threshold is required and must be a number")
	}

	in := &cloudwatch.PutMetricAlarmInput{
		AlarmName:          aws.String(alarmName),
		ComparisonOperator: cwtypes.ComparisonOperator(comparison),
		EvaluationPeriods:  aws.Int32(int32(evalPeriods)),
		Threshold:          aws.Float64(threshold),
	}
	if mn := strings.TrimSpace(awscommon.InputString("metric_name", inputs)); mn != "" {
		in.MetricName = aws.String(mn)
	}
	if ns := strings.TrimSpace(awscommon.InputString("namespace", inputs)); ns != "" {
		in.Namespace = aws.String(ns)
	}
	if p, ok := awscommon.InputInt("period", inputs); ok && p > 0 {
		in.Period = aws.Int32(int32(p))
	}
	if stat := strings.TrimSpace(awscommon.InputString("statistic", inputs)); stat != "" {
		in.Statistic = cwtypes.Statistic(stat)
	}
	if actions := splitCSV(awscommon.InputString("alarm_actions", inputs)); len(actions) > 0 {
		in.AlarmActions = actions
	}
	if dims := buildDimensions(inputs); len(dims) > 0 {
		in.Dimensions = dims
	}
	if desc := strings.TrimSpace(awscommon.InputString("alarm_description", inputs)); desc != "" {
		in.AlarmDescription = aws.String(desc)
	}
	if tmd := strings.TrimSpace(awscommon.InputString("treat_missing_data", inputs)); tmd != "" {
		in.TreatMissingData = aws.String(tmd)
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := cloudwatch.NewFromConfig(cfg)

	if _, err := client.PutMetricAlarm(ctx, in); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Alarm %s created/updated", alarmName),
		"alarm_name":  alarmName,
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

func buildDimensions(inputs []*core.Connection) []cwtypes.Dimension {
	conn := core.FindConnection("dimensions", inputs)
	if conn == nil {
		return nil
	}
	var dims []cwtypes.Dimension
	for _, kv := range conn.KeyValuePairs() {
		name := strings.TrimSpace(kv.Key)
		value := strings.TrimSpace(kv.Value)
		if name == "" || value == "" {
			continue
		}
		dims = append(dims, cwtypes.Dimension{Name: aws.String(name), Value: aws.String(value)})
	}
	return dims
}
