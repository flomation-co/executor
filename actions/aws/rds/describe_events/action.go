// Package aws_rds_describe_events lists RDS events, optionally narrowed by
// source, source type, or a lookback duration.
package aws_rds_describe_events

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Describe Events"
	Description  = "List RDS events, optionally filtered by source, type, or lookback duration."
	Website      = "https://www.flomation.co"
	Icon         = "bell+magnifying-glass"
	Date         = "20/07/2026"
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
	{Name: "source_identifier", Type: core.ConnectionTypeString, Label: "Source Identifier (optional)", Placeholder: "e.g. my-database"},
	{Name: "source_type", Type: core.ConnectionTypeString, Label: "Source Type (optional)", Options: []core.ConnectionOption{
		{Name: "Any", Value: ""},
		{Name: "DB Instance", Value: "db-instance"},
		{Name: "DB Cluster", Value: "db-cluster"},
		{Name: "DB Snapshot", Value: "db-snapshot"},
		{Name: "DB Parameter Group", Value: "db-parameter-group"},
		{Name: "DB Security Group", Value: "db-security-group"},
		{Name: "DB Cluster Snapshot", Value: "db-cluster-snapshot"},
	}},
	{Name: "duration", Type: core.ConnectionTypeInteger, Label: "Duration (minutes, optional)", Placeholder: "e.g. 1440"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "events", Type: core.ConnectionTypeObject, Label: "Events"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.DescribeEventsInput{}
	if s := awscommon.InputString("source_identifier", inputs); s != "" {
		in.SourceIdentifier = aws.String(s)
	}
	if s := awscommon.InputString("source_type", inputs); s != "" {
		in.SourceType = rdstypes.SourceType(s)
	}
	if d, ok := awscommon.InputInt("duration", inputs); ok {
		in.Duration = aws.Int32(int32(d))
	}

	var events []map[string]interface{}
	paginator := rds.NewDescribeEventsPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for i := range page.Events {
			e := &page.Events[i]
			m := map[string]interface{}{
				"source_identifier": aws.ToString(e.SourceIdentifier),
				"source_type":       string(e.SourceType),
				"message":           aws.ToString(e.Message),
				"event_categories":  e.EventCategories,
			}
			if e.Date != nil {
				m["date"] = e.Date.UTC().Format("2006-01-02T15:04:05Z")
			}
			events = append(events, m)
		}
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d event(s)", len(events)),
		"events":      events,
		"count":       len(events),
	}, nil
}
