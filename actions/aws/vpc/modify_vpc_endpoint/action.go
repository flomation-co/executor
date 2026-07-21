// Package aws_vpc_modify_vpc_endpoint modifies attributes of a VPC endpoint.
package aws_vpc_modify_vpc_endpoint

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
	Name         = "AWS VPC Modify VPC Endpoint"
	Description  = "Modify a VPC endpoint's route tables, subnets, or private DNS setting."
	Website      = "https://www.flomation.co"
	Icon         = "link+pen"
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
	{Name: "vpc_endpoint_id", Type: core.ConnectionTypeString, Label: "VPC Endpoint ID", Placeholder: "vpce-0abc", Required: true},
	{Name: "add_route_table_ids", Type: core.ConnectionTypeString, Label: "Add Route Table IDs", Placeholder: "Comma-separated, e.g. rtb-0abc"},
	{Name: "remove_route_table_ids", Type: core.ConnectionTypeString, Label: "Remove Route Table IDs", Placeholder: "Comma-separated, e.g. rtb-0abc"},
	{Name: "add_subnet_ids", Type: core.ConnectionTypeString, Label: "Add Subnet IDs", Placeholder: "Comma-separated, e.g. subnet-0abc"},
	{Name: "remove_subnet_ids", Type: core.ConnectionTypeString, Label: "Remove Subnet IDs", Placeholder: "Comma-separated, e.g. subnet-0abc"},
	{Name: "private_dns_enabled", Type: core.ConnectionTypeBoolean, Label: "Enable Private DNS (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	id := strings.TrimSpace(awscommon.InputString("vpc_endpoint_id", inputs))
	if id == "" {
		return nil, fmt.Errorf("vpc_endpoint_id is required")
	}

	in := &ec2.ModifyVpcEndpointInput{VpcEndpointId: aws.String(id)}
	changed := false
	if v := awscommon.InputStrings("add_route_table_ids", inputs); len(v) > 0 {
		in.AddRouteTableIds = v
		changed = true
	}
	if v := awscommon.InputStrings("remove_route_table_ids", inputs); len(v) > 0 {
		in.RemoveRouteTableIds = v
		changed = true
	}
	if v := awscommon.InputStrings("add_subnet_ids", inputs); len(v) > 0 {
		in.AddSubnetIds = v
		changed = true
	}
	if v := awscommon.InputStrings("remove_subnet_ids", inputs); len(v) > 0 {
		in.RemoveSubnetIds = v
		changed = true
	}
	if c := core.FindConnection("private_dns_enabled", inputs); c != nil {
		if b := c.Boolean(); b != nil {
			in.PrivateDnsEnabled = aws.Bool(*b)
			changed = true
		}
	}
	if !changed {
		return nil, fmt.Errorf("provide at least one change (route tables, subnets, or private DNS)")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	if _, err := client.ModifyVpcEndpoint(ctx, in); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Modified VPC endpoint %s", id),
	}, nil
}
