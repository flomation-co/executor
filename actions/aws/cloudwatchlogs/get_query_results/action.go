// Package aws_cloudwatchlogs_get_query_results fetches CloudWatch Logs Insights query results.
package aws_cloudwatchlogs_get_query_results

import (
	"context"
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS CloudWatch Get Query Results"
	Description  = "Fetch the results and status of a CloudWatch Logs Insights query."
	Website      = "https://www.flomation.co"
	Icon         = "file-lines+magnifying-glass"
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
	{Name: "query_id", Type: core.ConnectionTypeString, Label: "Query ID", Placeholder: "From Start Query", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "results", Type: core.ConnectionTypeString, Label: "Results (JSON)"},
	{Name: "statistics", Type: core.ConnectionTypeString, Label: "Statistics (JSON)"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	queryID := awscommon.InputString("query_id", inputs)
	if queryID == "" {
		return nil, fmt.Errorf("query id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := cloudwatchlogs.NewFromConfig(cfg)

	out, err := client.GetQueryResults(ctx, &cloudwatchlogs.GetQueryResultsInput{
		QueryId: aws.String(queryID),
	})
	if err != nil {
		return nil, err
	}

	type field struct {
		Field string `json:"field"`
		Value string `json:"value"`
	}
	rows := make([][]field, 0, len(out.Results))
	for _, row := range out.Results {
		fields := make([]field, 0, len(row))
		for _, f := range row {
			fields = append(fields, field{
				Field: aws.ToString(f.Field),
				Value: aws.ToString(f.Value),
			})
		}
		rows = append(rows, fields)
	}

	resultsJSON, err := json.Marshal(rows)
	if err != nil {
		return nil, fmt.Errorf("marshal results: %w", err)
	}

	var statsJSON []byte
	if out.Statistics != nil {
		statsJSON, err = json.Marshal(map[string]float64{
			"bytes_scanned":   out.Statistics.BytesScanned,
			"records_matched": out.Statistics.RecordsMatched,
			"records_scanned": out.Statistics.RecordsScanned,
		})
		if err != nil {
			return nil, fmt.Errorf("marshal statistics: %w", err)
		}
	} else {
		statsJSON = []byte("{}")
	}

	status := string(out.Status)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Query %s is %s with %d row(s)", queryID, status, len(rows)),
		"status":      status,
		"results":     string(resultsJSON),
		"statistics":  string(statsJSON),
	}, nil
}
