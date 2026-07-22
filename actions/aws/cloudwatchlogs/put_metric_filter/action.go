// Package aws_cloudwatchlogs_put_metric_filter creates or updates a CloudWatch Logs metric filter.
package aws_cloudwatchlogs_put_metric_filter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS CloudWatch Put Metric Filter"
	Description  = "Create or update a metric filter that emits CloudWatch metrics from log events."
	Website      = "https://www.flomation.co"
	Icon         = "file-lines+chart-line"
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
	{Name: "log_group_name", Type: core.ConnectionTypeString, Label: "Log Group Name", Placeholder: "/flomation/app", Required: true},
	{Name: "filter_name", Type: core.ConnectionTypeString, Label: "Filter Name", Placeholder: "ErrorCount", Required: true},
	{Name: "filter_pattern", Type: core.ConnectionTypeString, Label: "Filter Pattern", Placeholder: "ERROR", Required: true},
	{Name: "metric_transformations", Type: core.ConnectionTypeString, Label: "Metric Transformations (JSON array)", Placeholder: `[{"metricName":"ErrorCount","metricNamespace":"Flomation","metricValue":"1"}]`, Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "filter_name", Type: core.ConnectionTypeString, Label: "Filter Name"},
}

type transformation struct {
	MetricName      string `json:"metricName"`
	MetricNamespace string `json:"metricNamespace"`
	MetricValue     string `json:"metricValue"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	logGroup := awscommon.InputString("log_group_name", inputs)
	if logGroup == "" {
		return nil, fmt.Errorf("log group name is required")
	}
	filterName := awscommon.InputString("filter_name", inputs)
	if filterName == "" {
		return nil, fmt.Errorf("filter name is required")
	}
	filterPattern := awscommon.InputString("filter_pattern", inputs)
	if filterPattern == "" {
		return nil, fmt.Errorf("filter pattern is required")
	}
	transformsRaw := awscommon.InputString("metric_transformations", inputs)
	if strings.TrimSpace(transformsRaw) == "" {
		return nil, fmt.Errorf("metric transformations is required")
	}

	var parsed []transformation
	if err := json.Unmarshal([]byte(transformsRaw), &parsed); err != nil {
		return nil, fmt.Errorf("metric_transformations must be a JSON array of {metricName, metricNamespace, metricValue}: %w", err)
	}
	if len(parsed) == 0 {
		return nil, fmt.Errorf("metric_transformations array is empty")
	}
	transforms := make([]cwlogstypes.MetricTransformation, 0, len(parsed))
	for i, t := range parsed {
		if t.MetricName == "" || t.MetricNamespace == "" || t.MetricValue == "" {
			return nil, fmt.Errorf("metric transformation %d must set metricName, metricNamespace and metricValue", i)
		}
		transforms = append(transforms, cwlogstypes.MetricTransformation{
			MetricName:      aws.String(t.MetricName),
			MetricNamespace: aws.String(t.MetricNamespace),
			MetricValue:     aws.String(t.MetricValue),
		})
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := cloudwatchlogs.NewFromConfig(cfg)

	_, err = client.PutMetricFilter(ctx, &cloudwatchlogs.PutMetricFilterInput{
		LogGroupName:          aws.String(logGroup),
		FilterName:            aws.String(filterName),
		FilterPattern:         aws.String(filterPattern),
		MetricTransformations: transforms,
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Put metric filter %s on %s", filterName, logGroup),
		"filter_name": filterName,
	}, nil
}
