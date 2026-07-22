// Package aws_cloudwatch_put_metric_data publishes custom metric data points
// to Amazon CloudWatch.
package aws_cloudwatch_put_metric_data

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
	Name         = "AWS CloudWatch Put Metric Data"
	Description  = "Publish one or more custom metric data points to CloudWatch."
	Website      = "https://www.flomation.co"
	Icon         = "chart-line+plus"
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
	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "MyApp/Orders", Required: true},
	{Name: "metric_name", Type: core.ConnectionTypeString, Label: "Metric Name", Placeholder: "OrdersProcessed"},
	{Name: "value", Type: core.ConnectionTypeString, Label: "Value", Placeholder: "1"},
	{Name: "unit", Type: core.ConnectionTypeString, Label: "Unit", Options: []core.ConnectionOption{
		{Name: "None", Value: "None"},
		{Name: "Seconds", Value: "Seconds"},
		{Name: "Bytes", Value: "Bytes"},
		{Name: "Percent", Value: "Percent"},
		{Name: "Count", Value: "Count"},
		{Name: "Count/Second", Value: "Count/Second"},
		{Name: "Milliseconds", Value: "Milliseconds"},
	}},
	{Name: "dimensions", Type: core.ConnectionTypeKeyValueArray, Label: "Dimensions", Placeholder: "Add a Name and Value per dimension"},
	{Name: "metric_data", Type: core.ConnectionTypeString, Label: "Metric Data (JSON array, overrides simple fields)", Placeholder: `[{"MetricName":"Orders","Value":1,"Unit":"Count"}]`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "datapoint_count", Type: core.ConnectionTypeInteger, Label: "Data Points Sent"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	namespace := strings.TrimSpace(awscommon.InputString("namespace", inputs))
	if namespace == "" {
		return nil, fmt.Errorf("namespace is required")
	}

	var data []cwtypes.MetricDatum

	if raw := strings.TrimSpace(awscommon.InputString("metric_data", inputs)); raw != "" {
		if err := json.Unmarshal([]byte(raw), &data); err != nil {
			return nil, fmt.Errorf("metric_data must be a JSON array of metric data: %w", err)
		}
		if len(data) == 0 {
			return nil, fmt.Errorf("metric_data must contain at least one data point")
		}
	} else {
		metricName := strings.TrimSpace(awscommon.InputString("metric_name", inputs))
		if metricName == "" {
			return nil, fmt.Errorf("metric_name is required (or provide a metric_data JSON array)")
		}
		datum := cwtypes.MetricDatum{MetricName: aws.String(metricName)}
		if v, ok := awscommon.InputFloat("value", inputs); ok {
			datum.Value = aws.Float64(v)
		}
		if u := strings.TrimSpace(awscommon.InputString("unit", inputs)); u != "" {
			datum.Unit = cwtypes.StandardUnit(u)
		}
		if dims := buildDimensions(inputs); len(dims) > 0 {
			datum.Dimensions = dims
		}
		data = []cwtypes.MetricDatum{datum}
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := cloudwatch.NewFromConfig(cfg)

	_, err = client.PutMetricData(ctx, &cloudwatch.PutMetricDataInput{
		Namespace:  aws.String(namespace),
		MetricData: data,
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Published %d data point(s) to namespace %s", len(data), namespace),
		"datapoint_count": len(data),
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
