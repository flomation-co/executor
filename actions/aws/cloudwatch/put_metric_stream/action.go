// Package aws_cloudwatch_put_metric_stream creates or updates a CloudWatch
// metric stream that delivers metrics to a Kinesis Data Firehose.
package aws_cloudwatch_put_metric_stream

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
	Name         = "AWS CloudWatch Put Metric Stream"
	Description  = "Create or update a metric stream delivering metrics to Kinesis Data Firehose."
	Website      = "https://www.flomation.co"
	Icon         = "chart-line+arrow-up"
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
	{Name: "name", Type: core.ConnectionTypeString, Label: "Stream Name", Placeholder: "my-metric-stream", Required: true},
	{Name: "firehose_arn", Type: core.ConnectionTypeString, Label: "Firehose ARN", Placeholder: "arn:aws:firehose:eu-west-2:123456789012:deliverystream/my-stream", Required: true},
	{Name: "role_arn", Type: core.ConnectionTypeString, Label: "IAM Role ARN", Placeholder: "arn:aws:iam::123456789012:role/MetricStreamRole", Required: true},
	{Name: "output_format", Type: core.ConnectionTypeString, Label: "Output Format", Required: true, Options: []core.ConnectionOption{
		{Name: "JSON", Value: "json"},
		{Name: "OpenTelemetry 0.7", Value: "opentelemetry0.7"},
		{Name: "OpenTelemetry 1.0", Value: "opentelemetry1.0"},
	}},
	{Name: "include_filters", Type: core.ConnectionTypeString, Label: "Include Filters (JSON array, optional)", Placeholder: `[{"namespace":"AWS/EC2","metric_names":["CPUUtilization"]}]`},
	{Name: "exclude_filters", Type: core.ConnectionTypeString, Label: "Exclude Filters (JSON array, optional)", Placeholder: `[{"namespace":"AWS/Logs"}]`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "arn", Type: core.ConnectionTypeString, Label: "Stream ARN"},
}

// streamFilter is the curated JSON shape for a metric stream filter.
type streamFilter struct {
	Namespace   string   `json:"namespace"`
	MetricNames []string `json:"metric_names"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	name := strings.TrimSpace(awscommon.InputString("name", inputs))
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	firehoseArn := strings.TrimSpace(awscommon.InputString("firehose_arn", inputs))
	if firehoseArn == "" {
		return nil, fmt.Errorf("firehose_arn is required")
	}
	roleArn := strings.TrimSpace(awscommon.InputString("role_arn", inputs))
	if roleArn == "" {
		return nil, fmt.Errorf("role_arn is required")
	}
	outputFormat := strings.TrimSpace(awscommon.InputString("output_format", inputs))
	if outputFormat == "" {
		return nil, fmt.Errorf("output_format is required")
	}

	includeFilters, err := parseFilters("include_filters", inputs)
	if err != nil {
		return nil, err
	}
	excludeFilters, err := parseFilters("exclude_filters", inputs)
	if err != nil {
		return nil, err
	}

	in := &cloudwatch.PutMetricStreamInput{
		Name:         aws.String(name),
		FirehoseArn:  aws.String(firehoseArn),
		RoleArn:      aws.String(roleArn),
		OutputFormat: cwtypes.MetricStreamOutputFormat(outputFormat),
	}
	if len(includeFilters) > 0 {
		in.IncludeFilters = includeFilters
	}
	if len(excludeFilters) > 0 {
		in.ExcludeFilters = excludeFilters
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := cloudwatch.NewFromConfig(cfg)

	out, err := client.PutMetricStream(ctx, in)
	if err != nil {
		return nil, err
	}

	arn := aws.ToString(out.Arn)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Metric stream %s created/updated", name),
		"arn":         arn,
	}, nil
}

func parseFilters(field string, inputs []*core.Connection) ([]cwtypes.MetricStreamFilter, error) {
	raw := strings.TrimSpace(awscommon.InputString(field, inputs))
	if raw == "" {
		return nil, nil
	}
	var parsed []streamFilter
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("%s must be a JSON array: %w", field, err)
	}
	filters := make([]cwtypes.MetricStreamFilter, 0, len(parsed))
	for _, f := range parsed {
		ns := strings.TrimSpace(f.Namespace)
		if ns == "" {
			continue
		}
		filter := cwtypes.MetricStreamFilter{Namespace: aws.String(ns)}
		if len(f.MetricNames) > 0 {
			filter.MetricNames = f.MetricNames
		}
		filters = append(filters, filter)
	}
	return filters, nil
}
