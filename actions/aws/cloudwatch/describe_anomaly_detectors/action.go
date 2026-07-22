// Package aws_cloudwatch_describe_anomaly_detectors lists CloudWatch anomaly
// detection models, optionally filtered by namespace and metric.
package aws_cloudwatch_describe_anomaly_detectors

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS CloudWatch Describe Anomaly Detectors"
	Description  = "List anomaly detection models, filtered by namespace/metric."
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
	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace (optional)", Placeholder: "AWS/EC2"},
	{Name: "metric_name", Type: core.ConnectionTypeString, Label: "Metric Name (optional)", Placeholder: "CPUUtilization"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "detectors", Type: core.ConnectionTypeString, Label: "Detectors (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Detector count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	in := &cloudwatch.DescribeAnomalyDetectorsInput{}
	if ns := strings.TrimSpace(awscommon.InputString("namespace", inputs)); ns != "" {
		in.Namespace = aws.String(ns)
	}
	if mn := strings.TrimSpace(awscommon.InputString("metric_name", inputs)); mn != "" {
		in.MetricName = aws.String(mn)
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := cloudwatch.NewFromConfig(cfg)

	out, err := client.DescribeAnomalyDetectors(ctx, in)
	if err != nil {
		return nil, err
	}

	detectorsJSON, err := json.Marshal(out.AnomalyDetectors)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d anomaly detector(s)", len(out.AnomalyDetectors)),
		"detectors":   string(detectorsJSON),
		"count":       len(out.AnomalyDetectors),
	}, nil
}
