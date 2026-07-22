// Package aws_cloudwatch_get_metric_widget_image renders a CloudWatch metric
// widget definition into a PNG image.
package aws_cloudwatch_get_metric_widget_image

import (
	"context"
	"encoding/base64"
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
	Name         = "AWS CloudWatch Get Metric Widget Image"
	Description  = "Render a metric-widget definition into a PNG graph image."
	Website      = "https://www.flomation.co"
	Icon         = "chart-line+image"
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
	{Name: "metric_widget", Type: core.ConnectionTypeString, Label: "Metric Widget (JSON)", Placeholder: `{"metrics":[["AWS/EC2","CPUUtilization"]]}`, Required: true},
	{Name: "output_format", Type: core.ConnectionTypeString, Label: "Output Format", Placeholder: "png"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "image_base64", Type: core.ConnectionTypeString, Label: "PNG image (base64)"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	widget := strings.TrimSpace(awscommon.InputString("metric_widget", inputs))
	if widget == "" {
		return nil, fmt.Errorf("metric_widget is required")
	}

	in := &cloudwatch.GetMetricWidgetImageInput{
		MetricWidget: aws.String(widget),
	}
	if format := strings.TrimSpace(awscommon.InputString("output_format", inputs)); format != "" {
		in.OutputFormat = aws.String(format)
	} else {
		in.OutputFormat = aws.String("png")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := cloudwatch.NewFromConfig(cfg)

	out, err := client.GetMetricWidgetImage(ctx, in)
	if err != nil {
		return nil, err
	}

	encoded := base64.StdEncoding.EncodeToString(out.MetricWidgetImage)

	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Rendered metric widget image (%d bytes)", len(out.MetricWidgetImage)),
		"image_base64": encoded,
	}, nil
}
