// Package aws_cloudwatch_list_dashboards lists CloudWatch dashboards.
package aws_cloudwatch_list_dashboards

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS CloudWatch List Dashboards"
	Description  = "List CloudWatch dashboards, optionally filtered by a name prefix."
	Website      = "https://www.flomation.co"
	Icon         = "grip+list"
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
	{Name: "dashboard_name_prefix", Type: core.ConnectionTypeString, Label: "Dashboard Name Prefix (optional)", Placeholder: "prod-"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "dashboards", Type: core.ConnectionTypeString, Label: "Dashboards (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := cloudwatch.NewFromConfig(cfg)

	in := &cloudwatch.ListDashboardsInput{}
	if prefix := awscommon.InputString("dashboard_name_prefix", inputs); prefix != "" {
		in.DashboardNamePrefix = aws.String(prefix)
	}

	type dashboardInfo struct {
		Name         string `json:"name"`
		LastModified string `json:"last_modified"`
	}
	var dashboards []dashboardInfo

	for {
		out, err := client.ListDashboards(ctx, in)
		if err != nil {
			return nil, err
		}
		for _, d := range out.DashboardEntries {
			lm := ""
			if d.LastModified != nil {
				lm = d.LastModified.UTC().Format(time.RFC3339)
			}
			dashboards = append(dashboards, dashboardInfo{
				Name:         aws.ToString(d.DashboardName),
				LastModified: lm,
			})
		}
		if out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		in.NextToken = out.NextToken
	}

	dashboardsJSON := "[]"
	if b, mErr := json.Marshal(dashboards); mErr == nil {
		dashboardsJSON = string(b)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d dashboard(s)", len(dashboards)),
		"dashboards":  dashboardsJSON,
		"count":       len(dashboards),
	}, nil
}
