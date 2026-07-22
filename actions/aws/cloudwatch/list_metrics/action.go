// Package aws_cloudwatch_list_metrics lists the CloudWatch metrics available in
// an account, optionally filtered by namespace, name, and dimensions.
package aws_cloudwatch_list_metrics

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
	Name         = "AWS CloudWatch List Metrics"
	Description  = "List available CloudWatch metrics, optionally filtered."
	Website      = "https://www.flomation.co"
	Icon         = "chart-line+list"
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
	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace (optional)", Placeholder: "AWS/EC2"},
	{Name: "metric_name", Type: core.ConnectionTypeString, Label: "Metric Name (optional)", Placeholder: "CPUUtilization"},
	{Name: "dimensions", Type: core.ConnectionTypeKeyValueArray, Label: "Dimension Filters", Placeholder: "Add a Name and (optional) Value per filter"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "metrics", Type: core.ConnectionTypeString, Label: "Metrics (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Metric Count"},
}

type metricSummary struct {
	Namespace  string            `json:"namespace"`
	MetricName string            `json:"metric_name"`
	Dimensions map[string]string `json:"dimensions"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := cloudwatch.NewFromConfig(cfg)

	in := &cloudwatch.ListMetricsInput{}
	if ns := strings.TrimSpace(awscommon.InputString("namespace", inputs)); ns != "" {
		in.Namespace = aws.String(ns)
	}
	if mn := strings.TrimSpace(awscommon.InputString("metric_name", inputs)); mn != "" {
		in.MetricName = aws.String(mn)
	}
	if filters := buildDimensionFilters(inputs); len(filters) > 0 {
		in.Dimensions = filters
	}

	var summaries []metricSummary
	paginator := cloudwatch.NewListMetricsPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, m := range page.Metrics {
			dims := make(map[string]string)
			for _, d := range m.Dimensions {
				dims[aws.ToString(d.Name)] = aws.ToString(d.Value)
			}
			summaries = append(summaries, metricSummary{
				Namespace:  aws.ToString(m.Namespace),
				MetricName: aws.ToString(m.MetricName),
				Dimensions: dims,
			})
		}
	}

	encoded, err := json.Marshal(summaries)
	if err != nil {
		return nil, fmt.Errorf("encode metrics: %w", err)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d metric(s)", len(summaries)),
		"metrics":     string(encoded),
		"count":       len(summaries),
	}, nil
}

func buildDimensionFilters(inputs []*core.Connection) []cwtypes.DimensionFilter {
	conn := core.FindConnection("dimensions", inputs)
	if conn == nil {
		return nil
	}
	var filters []cwtypes.DimensionFilter
	for _, kv := range conn.KeyValuePairs() {
		name := strings.TrimSpace(kv.Key)
		if name == "" {
			continue
		}
		f := cwtypes.DimensionFilter{Name: aws.String(name)}
		if value := strings.TrimSpace(kv.Value); value != "" {
			f.Value = aws.String(value)
		}
		filters = append(filters, f)
	}
	return filters
}
