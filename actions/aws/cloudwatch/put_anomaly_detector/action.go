// Package aws_cloudwatch_put_anomaly_detector creates a single-metric
// CloudWatch anomaly detection model.
package aws_cloudwatch_put_anomaly_detector

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
	Name         = "AWS CloudWatch Put Anomaly Detector"
	Description  = "Create a single-metric anomaly detection model."
	Website      = "https://www.flomation.co"
	Icon         = "chart-line+bolt"
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
	{Name: "stat", Type: core.ConnectionTypeString, Label: "Statistic", Placeholder: "Average", Required: true, Options: []core.ConnectionOption{
		{Name: "Average", Value: "Average"},
		{Name: "Sum", Value: "Sum"},
		{Name: "Minimum", Value: "Minimum"},
		{Name: "Maximum", Value: "Maximum"},
		{Name: "Sample Count", Value: "SampleCount"},
	}},
	{Name: "dimensions", Type: core.ConnectionTypeKeyValueArray, Label: "Dimensions", Placeholder: "Add a Name and Value per dimension"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
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
	stat := strings.TrimSpace(awscommon.InputString("stat", inputs))
	if stat == "" {
		return nil, fmt.Errorf("stat is required")
	}

	detector := &cwtypes.SingleMetricAnomalyDetector{
		Namespace:  aws.String(namespace),
		MetricName: aws.String(metricName),
		Stat:       aws.String(stat),
	}
	if dims := buildDimensions(inputs); len(dims) > 0 {
		detector.Dimensions = dims
	}

	in := &cloudwatch.PutAnomalyDetectorInput{
		SingleMetricAnomalyDetector: detector,
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := cloudwatch.NewFromConfig(cfg)

	if _, err := client.PutAnomalyDetector(ctx, in); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Anomaly detector created for %s/%s (%s)", namespace, metricName, stat),
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
