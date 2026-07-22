// Package aws_cloudwatch_get_metric_data retrieves metric data or math
// expression results from CloudWatch via metric data queries.
package aws_cloudwatch_get_metric_data

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
	Name         = "AWS CloudWatch Get Metric Data"
	Description  = "Fetch metric data or math expression results via metric data queries."
	Website      = "https://www.flomation.co"
	Icon         = "chart-area+magnifying-glass"
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
	{Name: "metric_data_queries", Type: core.ConnectionTypeString, Label: "Metric Data Queries (JSON array)", Placeholder: `[{"Id":"m1","MetricStat":{"Metric":{"Namespace":"AWS/EC2","MetricName":"CPUUtilization"},"Period":300,"Stat":"Average"}}]`, Required: true},
	{Name: "start_time", Type: core.ConnectionTypeString, Label: "Start Time (RFC3339)", Placeholder: "2026-07-22T00:00:00Z", Required: true},
	{Name: "end_time", Type: core.ConnectionTypeString, Label: "End Time (RFC3339)", Placeholder: "2026-07-22T01:00:00Z", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "results", Type: core.ConnectionTypeString, Label: "Results (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Result Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	queriesRaw := strings.TrimSpace(awscommon.InputString("metric_data_queries", inputs))
	if queriesRaw == "" {
		return nil, fmt.Errorf("metric_data_queries JSON array is required")
	}
	var queries []cwtypes.MetricDataQuery
	if err := json.Unmarshal([]byte(queriesRaw), &queries); err != nil {
		return nil, fmt.Errorf("metric_data_queries must be a JSON array of metric data queries: %w", err)
	}
	if len(queries) == 0 {
		return nil, fmt.Errorf("at least one metric data query is required")
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

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := cloudwatch.NewFromConfig(cfg)

	out, err := client.GetMetricData(ctx, &cloudwatch.GetMetricDataInput{
		MetricDataQueries: queries,
		StartTime:         aws.Time(startTime),
		EndTime:           aws.Time(endTime),
	})
	if err != nil {
		return nil, err
	}

	encoded, err := json.Marshal(out.MetricDataResults)
	if err != nil {
		return nil, fmt.Errorf("encode results: %w", err)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Retrieved %d metric data result(s)", len(out.MetricDataResults)),
		"results":     string(encoded),
		"count":       len(out.MetricDataResults),
	}, nil
}
