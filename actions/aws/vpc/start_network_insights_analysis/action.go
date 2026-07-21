// Package aws_vpc_start_network_insights_analysis runs a Reachability Analyzer analysis.
package aws_vpc_start_network_insights_analysis

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/google/uuid"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS VPC Start Network Insights Analysis"
	Description  = "Run a Reachability Analyzer analysis for a network insights path."
	Website      = "https://www.flomation.co"
	Icon         = "route+play"
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
	{Name: "network_insights_path_id", Type: core.ConnectionTypeString, Label: "Path ID", Required: true},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "analysis", Type: core.ConnectionTypeObject, Label: "Network Insights Analysis"},
	{Name: "network_insights_analysis_id", Type: core.ConnectionTypeString, Label: "Analysis ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	pathID := strings.TrimSpace(awscommon.InputString("network_insights_path_id", inputs))
	if pathID == "" {
		return nil, fmt.Errorf("network_insights_path_id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.StartNetworkInsightsAnalysisInput{
		ClientToken:           aws.String(uuid.NewString()),
		NetworkInsightsPathId: aws.String(pathID),
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.TagSpecifications = []ec2types.TagSpecification{{
			ResourceType: ec2types.ResourceTypeNetworkInsightsAnalysis,
			Tags:         tags,
		}}
	}

	out, err := client.StartNetworkInsightsAnalysis(ctx, in)
	if err != nil {
		return nil, err
	}

	analysis := map[string]interface{}{}
	id := ""
	if out.NetworkInsightsAnalysis != nil {
		a := out.NetworkInsightsAnalysis
		id = aws.ToString(a.NetworkInsightsAnalysisId)
		analysis = map[string]interface{}{
			"network_insights_analysis_id":  id,
			"network_insights_analysis_arn": aws.ToString(a.NetworkInsightsAnalysisArn),
			"network_insights_path_id":      aws.ToString(a.NetworkInsightsPathId),
			"status":                        string(a.Status),
			"network_path_found":            aws.ToBool(a.NetworkPathFound),
		}
		if a.StartDate != nil {
			analysis["start_date"] = a.StartDate.UTC().Format("2006-01-02T15:04:05Z")
		}
	}

	return map[string]interface{}{
		"tool_result":                  fmt.Sprintf("Started network insights analysis %s", id),
		"analysis":                     analysis,
		"network_insights_analysis_id": id,
	}, nil
}

func buildTags(inputs []*core.Connection) []ec2types.Tag {
	conn := core.FindConnection("tags", inputs)
	if conn == nil {
		return nil
	}
	var tags []ec2types.Tag
	for _, kv := range conn.KeyValuePairs() {
		k := strings.TrimSpace(kv.Key)
		if k == "" {
			continue
		}
		tags = append(tags, ec2types.Tag{Key: aws.String(k), Value: aws.String(kv.Value)})
	}
	return tags
}
