// Package aws_rds_describe_reserved_db_instances lists the purchased reserved
// RDS DB instances in the account.
package aws_rds_describe_reserved_db_instances

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Describe Reserved DB Instances"
	Description  = "List purchased reserved RDS DB instances, optionally filtered."
	Website      = "https://www.flomation.co"
	Icon         = "calendar+magnifying-glass"
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
	{Name: "db_instance_class", Type: core.ConnectionTypeString, Label: "DB Instance Class (optional)", Placeholder: "e.g. db.t3.medium"},
	{Name: "duration", Type: core.ConnectionTypeString, Label: "Duration (optional)", Placeholder: "Seconds or years, e.g. 1 or 31536000"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "reserved_instances", Type: core.ConnectionTypeObject, Label: "Reserved Instances"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.DescribeReservedDBInstancesInput{}
	if c := awscommon.InputString("db_instance_class", inputs); c != "" {
		in.DBInstanceClass = aws.String(c)
	}
	if d := awscommon.InputString("duration", inputs); d != "" {
		in.Duration = aws.String(d)
	}

	var reserved []map[string]interface{}
	paginator := rds.NewDescribeReservedDBInstancesPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for i := range page.ReservedDBInstances {
			r := &page.ReservedDBInstances[i]
			m := map[string]interface{}{
				"reserved_db_instance_id": aws.ToString(r.ReservedDBInstanceId),
				"db_instance_class":       aws.ToString(r.DBInstanceClass),
				"state":                   aws.ToString(r.State),
				"db_instance_count":       aws.ToInt32(r.DBInstanceCount),
			}
			if r.StartTime != nil {
				m["start_time"] = r.StartTime.UTC().Format("2006-01-02T15:04:05Z")
			}
			reserved = append(reserved, m)
		}
	}

	return map[string]interface{}{
		"tool_result":        fmt.Sprintf("Found %d reserved DB instance(s)", len(reserved)),
		"reserved_instances": reserved,
		"count":              len(reserved),
	}, nil
}
