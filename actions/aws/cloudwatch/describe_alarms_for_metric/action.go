// Package aws_cloudwatch_describe_alarms_for_metric lists the alarms
// associated with a specific CloudWatch metric.
package aws_cloudwatch_describe_alarms_for_metric

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
	Name         = "AWS CloudWatch Describe Alarms For Metric"
	Description  = "List the alarms associated with a specific metric."
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
	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "AWS/EC2", Required: true},
	{Name: "metric_name", Type: core.ConnectionTypeString, Label: "Metric Name", Placeholder: "CPUUtilization", Required: true},
	{Name: "dimensions", Type: core.ConnectionTypeKeyValueArray, Label: "Dimensions", Placeholder: "Add a Name and Value per dimension"},
	{Name: "statistic", Type: core.ConnectionTypeString, Label: "Statistic", Options: []core.ConnectionOption{
		{Name: "Average", Value: "Average"},
		{Name: "Sum", Value: "Sum"},
		{Name: "Minimum", Value: "Minimum"},
		{Name: "Maximum", Value: "Maximum"},
		{Name: "Sample Count", Value: "SampleCount"},
	}},
	{Name: "period", Type: core.ConnectionTypeInteger, Label: "Period (seconds)", Placeholder: "300"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "alarms", Type: core.ConnectionTypeString, Label: "Alarms (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Alarm count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	namespace := strings.TrimSpace(awscommon.InputString("namespace", inputs))
	if namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}
	metricName := strings.TrimSpace(awscommon.InputString("metric_name", inputs))
	if metricName == "" {
		return nil, fmt.Errorf("metric_name is required")
	}

	in := &cloudwatch.DescribeAlarmsForMetricInput{
		Namespace:  aws.String(namespace),
		MetricName: aws.String(metricName),
	}
	if dims := buildDimensions(inputs); len(dims) > 0 {
		in.Dimensions = dims
	}
	if stat := strings.TrimSpace(awscommon.InputString("statistic", inputs)); stat != "" {
		in.Statistic = cwtypes.Statistic(stat)
	}
	if p, ok := awscommon.InputInt("period", inputs); ok && p > 0 {
		in.Period = aws.Int32(int32(p))
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := cloudwatch.NewFromConfig(cfg)

	out, err := client.DescribeAlarmsForMetric(ctx, in)
	if err != nil {
		return nil, err
	}

	alarmsJSON, err := json.Marshal(out.MetricAlarms)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d alarm(s) for %s/%s", len(out.MetricAlarms), namespace, metricName),
		"alarms":      string(alarmsJSON),
		"count":       len(out.MetricAlarms),
	}, nil
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
