// Package aws_cloudwatch_describe_alarm_history retrieves the history of state
// changes and configuration updates for CloudWatch alarms.
package aws_cloudwatch_describe_alarm_history

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
	Name         = "AWS CloudWatch Describe Alarm History"
	Description  = "Retrieve the state change and configuration history for alarms."
	Website      = "https://www.flomation.co"
	Icon         = "bell+clock"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

const maxHistoryItems = 50

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
	{Name: "alarm_name", Type: core.ConnectionTypeString, Label: "Alarm Name (optional)", Placeholder: "high-cpu"},
	{Name: "history_item_type", Type: core.ConnectionTypeString, Label: "History Item Type (optional)", Options: []core.ConnectionOption{
		{Name: "Configuration Update", Value: "ConfigurationUpdate"},
		{Name: "State Update", Value: "StateUpdate"},
		{Name: "Action", Value: "Action"},
	}},
	{Name: "start_date", Type: core.ConnectionTypeString, Label: "Start Date (RFC3339, optional)", Placeholder: "2026-07-22T00:00:00Z"},
	{Name: "end_date", Type: core.ConnectionTypeString, Label: "End Date (RFC3339, optional)", Placeholder: "2026-07-22T23:59:59Z"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "history", Type: core.ConnectionTypeString, Label: "History (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "History Item Count"},
}

type historyItem struct {
	AlarmName       string `json:"alarm_name"`
	HistoryItemType string `json:"history_item_type"`
	HistorySummary  string `json:"history_summary"`
	Timestamp       string `json:"timestamp"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	in := &cloudwatch.DescribeAlarmHistoryInput{}
	if name := strings.TrimSpace(awscommon.InputString("alarm_name", inputs)); name != "" {
		in.AlarmName = aws.String(name)
	}
	if hit := strings.TrimSpace(awscommon.InputString("history_item_type", inputs)); hit != "" {
		in.HistoryItemType = cwtypes.HistoryItemType(hit)
	}
	if sd := strings.TrimSpace(awscommon.InputString("start_date", inputs)); sd != "" {
		t, err := time.Parse(time.RFC3339, sd)
		if err != nil {
			return nil, fmt.Errorf("start_date must be RFC3339 (e.g. 2026-07-22T00:00:00Z): %w", err)
		}
		in.StartDate = aws.Time(t)
	}
	if ed := strings.TrimSpace(awscommon.InputString("end_date", inputs)); ed != "" {
		t, err := time.Parse(time.RFC3339, ed)
		if err != nil {
			return nil, fmt.Errorf("end_date must be RFC3339 (e.g. 2026-07-22T23:59:59Z): %w", err)
		}
		in.EndDate = aws.Time(t)
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := cloudwatch.NewFromConfig(cfg)

	var items []historyItem
	paginator := cloudwatch.NewDescribeAlarmHistoryPaginator(client, in)
	for paginator.HasMorePages() && len(items) < maxHistoryItems {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, h := range page.AlarmHistoryItems {
			ts := ""
			if h.Timestamp != nil {
				ts = h.Timestamp.Format(time.RFC3339)
			}
			items = append(items, historyItem{
				AlarmName:       aws.ToString(h.AlarmName),
				HistoryItemType: string(h.HistoryItemType),
				HistorySummary:  aws.ToString(h.HistorySummary),
				Timestamp:       ts,
			})
			if len(items) >= maxHistoryItems {
				break
			}
		}
	}

	encoded, err := json.Marshal(items)
	if err != nil {
		return nil, fmt.Errorf("encode history: %w", err)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Retrieved %d history item(s)", len(items)),
		"history":     string(encoded),
		"count":       len(items),
	}, nil
}
