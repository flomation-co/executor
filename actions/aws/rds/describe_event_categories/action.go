// Package aws_rds_describe_event_categories lists the event categories that RDS
// publishes for each source type.
package aws_rds_describe_event_categories

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Describe Event Categories"
	Description  = "List the RDS event categories available for each source type."
	Website      = "https://www.flomation.co"
	Icon         = "bell+list"
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
	{Name: "source_type", Type: core.ConnectionTypeString, Label: "Source Type (optional)", Options: []core.ConnectionOption{
		{Name: "All source types", Value: ""},
		{Name: "DB Instance", Value: "db-instance"},
		{Name: "DB Cluster", Value: "db-cluster"},
		{Name: "DB Snapshot", Value: "db-snapshot"},
		{Name: "DB Parameter Group", Value: "db-parameter-group"},
		{Name: "DB Security Group", Value: "db-security-group"},
		{Name: "DB Cluster Snapshot", Value: "db-cluster-snapshot"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "categories", Type: core.ConnectionTypeObject, Label: "Event Categories"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.DescribeEventCategoriesInput{}
	if v := awscommon.InputString("source_type", inputs); v != "" {
		in.SourceType = aws.String(v)
	}

	out, err := client.DescribeEventCategories(ctx, in)
	if err != nil {
		return nil, err
	}

	var categories []map[string]interface{}
	for _, m := range out.EventCategoriesMapList {
		categories = append(categories, map[string]interface{}{
			"source_type":      aws.ToString(m.SourceType),
			"event_categories": strings.Join(m.EventCategories, ", "),
		})
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d event category mapping(s)", len(categories)),
		"categories":  categories,
		"count":       len(categories),
	}, nil
}
