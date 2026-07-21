// Package aws_rds_create_db_subnet_group creates an RDS DB subnet group from a
// set of EC2 subnet IDs.
package aws_rds_create_db_subnet_group

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Create DB Subnet Group"
	Description  = "Create an RDS DB subnet group from a set of EC2 subnet IDs."
	Website      = "https://www.flomation.co"
	Icon         = "object-group+plus"
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
	{Name: "db_subnet_group_name", Type: core.ConnectionTypeString, Label: "DB Subnet Group Name", Placeholder: "my-subnet-group", Required: true},
	{Name: "db_subnet_group_description", Type: core.ConnectionTypeString, Label: "Description", Required: true},
	{Name: "subnet_ids", Type: core.ConnectionTypeString, Label: "Subnet IDs", Placeholder: "Comma-separated, e.g. subnet-abc123,subnet-def456", Required: true},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "subnet_group", Type: core.ConnectionTypeObject, Label: "DB Subnet Group"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	name := awscommon.InputString("db_subnet_group_name", inputs)
	description := awscommon.InputString("db_subnet_group_description", inputs)
	subnetIDs := awscommon.InputStrings("subnet_ids", inputs)
	if name == "" || description == "" || len(subnetIDs) == 0 {
		return nil, fmt.Errorf("db subnet group name, description and at least one subnet id are required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.CreateDBSubnetGroupInput{
		DBSubnetGroupName:        aws.String(name),
		DBSubnetGroupDescription: aws.String(description),
		SubnetIds:                subnetIDs,
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.Tags = tags
	}

	out, err := client.CreateDBSubnetGroup(ctx, in)
	if err != nil {
		return nil, err
	}

	sg := out.DBSubnetGroup
	var subnets []string
	for i := range sg.Subnets {
		subnets = append(subnets, aws.ToString(sg.Subnets[i].SubnetIdentifier))
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created DB subnet group %q (status: %s)", aws.ToString(sg.DBSubnetGroupName), aws.ToString(sg.SubnetGroupStatus)),
		"subnet_group": map[string]interface{}{
			"name":        aws.ToString(sg.DBSubnetGroupName),
			"description": aws.ToString(sg.DBSubnetGroupDescription),
			"vpc_id":      aws.ToString(sg.VpcId),
			"status":      aws.ToString(sg.SubnetGroupStatus),
			"subnet_ids":  subnets,
		},
	}, nil
}

func buildTags(inputs []*core.Connection) []rdstypes.Tag {
	conn := core.FindConnection("tags", inputs)
	if conn == nil {
		return nil
	}
	var tags []rdstypes.Tag
	for _, kv := range conn.KeyValuePairs() {
		k := strings.TrimSpace(kv.Key)
		if k == "" {
			continue
		}
		tags = append(tags, rdstypes.Tag{Key: aws.String(k), Value: aws.String(kv.Value)})
	}
	return tags
}
