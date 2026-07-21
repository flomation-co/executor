// Package aws_vpc_describe_network_insights_analyses lists Reachability Analyzer analyses.
package aws_vpc_describe_network_insights_analyses

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS VPC Describe Network Insights Analyses"
	Description  = "List Reachability Analyzer analyses, optionally filtered by analysis or path id."
	Website      = "https://www.flomation.co"
	Icon         = "route+magnifying-glass"
	Date         = "21/07/2026"
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
	{Name: "network_insights_analysis_id", Type: core.ConnectionTypeString, Label: "Analysis ID (optional)", Placeholder: "Leave blank to list all"},
	{Name: "network_insights_path_id", Type: core.ConnectionTypeString, Label: "Path ID (optional)", Placeholder: "Filter analyses to one path"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "analyses", Type: core.ConnectionTypeObject, Label: "Analyses"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.DescribeNetworkInsightsAnalysesInput{}
	if ids := awscommon.InputStrings("network_insights_analysis_id", inputs); len(ids) > 0 {
		in.NetworkInsightsAnalysisIds = ids
	}
	if p := strings.TrimSpace(awscommon.InputString("network_insights_path_id", inputs)); p != "" {
		in.NetworkInsightsPathId = aws.String(p)
	}

	var analyses []map[string]interface{}
	paginator := ec2.NewDescribeNetworkInsightsAnalysesPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for i := range page.NetworkInsightsAnalyses {
			a := &page.NetworkInsightsAnalyses[i]
			m := map[string]interface{}{
				"network_insights_analysis_id": aws.ToString(a.NetworkInsightsAnalysisId),
				"network_insights_path_id":     aws.ToString(a.NetworkInsightsPathId),
				"status":                       string(a.Status),
				"network_path_found":           aws.ToBool(a.NetworkPathFound),
			}
			if a.StartDate != nil {
				m["start_date"] = a.StartDate.UTC().Format("2006-01-02T15:04:05Z")
			}
			analyses = append(analyses, m)
		}
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d network insights analysis(es)", len(analyses)),
		"analyses":    analyses,
		"count":       len(analyses),
	}, nil
}
