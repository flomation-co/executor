// Package aws_cloudwatch_put_dashboard creates or updates a CloudWatch dashboard.
package aws_cloudwatch_put_dashboard

import (
	"context"
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS CloudWatch Put Dashboard"
	Description  = "Create or replace a CloudWatch dashboard from its widget layout JSON."
	Website      = "https://www.flomation.co"
	Icon         = "grip+plus"
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
	{Name: "dashboard_name", Type: core.ConnectionTypeString, Label: "Dashboard Name", Placeholder: "my-dashboard", Required: true},
	{Name: "dashboard_body", Type: core.ConnectionTypeString, Label: "Dashboard Body (JSON)", Placeholder: `{"widgets":[...]}`, Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "validation_messages", Type: core.ConnectionTypeString, Label: "Validation Messages (JSON)"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	name := awscommon.InputString("dashboard_name", inputs)
	if name == "" {
		return nil, fmt.Errorf("dashboard name is required")
	}
	body := awscommon.InputString("dashboard_body", inputs)
	if body == "" {
		return nil, fmt.Errorf("dashboard body is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := cloudwatch.NewFromConfig(cfg)

	out, err := client.PutDashboard(ctx, &cloudwatch.PutDashboardInput{
		DashboardName: aws.String(name),
		DashboardBody: aws.String(body),
	})
	if err != nil {
		return nil, err
	}

	validationJSON := ""
	if len(out.DashboardValidationMessages) > 0 {
		msgs := make([]map[string]string, 0, len(out.DashboardValidationMessages))
		for _, m := range out.DashboardValidationMessages {
			msgs = append(msgs, map[string]string{
				"data_path": aws.ToString(m.DataPath),
				"message":   aws.ToString(m.Message),
			})
		}
		if b, mErr := json.Marshal(msgs); mErr == nil {
			validationJSON = string(b)
		}
	}

	result := fmt.Sprintf("Saved dashboard %s", name)
	if validationJSON != "" {
		result = fmt.Sprintf("Saved dashboard %s with %d validation message(s)", name, len(out.DashboardValidationMessages))
	}

	return map[string]interface{}{
		"tool_result":         result,
		"validation_messages": validationJSON,
	}, nil
}
