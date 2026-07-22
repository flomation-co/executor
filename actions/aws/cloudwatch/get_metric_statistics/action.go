// Package aws_cloudwatch_get_metric_statistics retrieves aggregated statistics
// for a CloudWatch metric over a time range.
package aws_cloudwatch_get_metric_statistics

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS CloudWatch Get Metric Statistics"
	Description  = "Retrieve aggregated statistics for a metric over a time range."
	Website      = "https://www.flomation.co"
	Icon         = "chart-line+magnifying-glass"
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
	{Name: "start_time", Type: core.ConnectionTypeString, Label: "Start Time (RFC3339)", Placeholder: "2026-07-22T00:00:00Z", Required: true},
	{Name: "end_time", Type: core.ConnectionTypeString, Label: "End Time (RFC3339)", Placeholder: "2026-07-22T01:00:00Z", Required: true},
	{Name: "period", Type: core.ConnectionTypeInteger, Label: "Period (seconds)", Placeholder: "300", Required: true},
	{Name: "statistics", Type: core.ConnectionTypeString, Label: "Statistics (comma-separated)", Placeholder: "Average,Maximum", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "datapoints", Type: core.ConnectionTypeString, Label: "Data Points (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Data Point Count"},
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
	startStr := strings.TrimSpace(awscommon.InputString("start_time", inputs))
	if startStr == "" {
		return nil, fmt.Errorf("start_time is required")
	}
	endStr := strings.TrimSpace(awscommon.InputString("end_time", inputs))
	if endStr == "" {
		return nil, fmt.Errorf("end_time is required")
	}
	startTime, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		return nil, fmt.Errorf("start_time must be RFC3339 (e.g. 2026-07-22T00:00:00Z): %w", err)
	}
	endTime, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		return nil, fmt.Errorf("end_time must be RFC3339 (e.g. 2026-07-22T01:00:00Z): %w", err)
	}
	period, ok := awscommon.InputInt("period", inputs)
	if !ok || period <= 0 {
		return nil, fmt.Errorf("period (seconds) is required and must be positive")
	}
	statsRaw := strings.TrimSpace(awscommon.InputString("statistics", inputs))
	if statsRaw == "" {
		return nil, fmt.Errorf("statistics is required (e.g. Average,Maximum)")
	}
	var stats []cwtypes.Statistic
	for _, s := range strings.Split(statsRaw, ",") {
		if t := strings.TrimSpace(s); t != "" {
			stats = append(stats, cwtypes.Statistic(t))
		}
	}
	if len(stats) == 0 {
		return nil, fmt.Errorf("at least one statistic is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := cloudwatch.NewFromConfig(cfg)

	in := &cloudwatch.GetMetricStatisticsInput{
		Namespace:  aws.String(namespace),
		MetricName: aws.String(metricName),
		StartTime:  aws.Time(startTime),
		EndTime:    aws.Time(endTime),
		Period:     aws.Int32(int32(period)),
		Statistics: stats,
	}
	if dims := buildDimensions(inputs); len(dims) > 0 {
		in.Dimensions = dims
	}

	out, err := client.GetMetricStatistics(ctx, in)
	if err != nil {
		return nil, err
	}

	encoded, err := json.Marshal(out.Datapoints)
	if err != nil {
		return nil, fmt.Errorf("encode datapoints: %w", err)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Retrieved %d data point(s) for %s/%s", len(out.Datapoints), namespace, metricName),
		"datapoints":  string(encoded),
		"count":       len(out.Datapoints),
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
