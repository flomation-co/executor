// Package aws_vpc_create_transit_gateway_route_table creates a transit gateway route table.
package aws_vpc_create_transit_gateway_route_table

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS VPC Create Transit Gateway Route Table"
	Description  = "Create a transit gateway route table for custom routing domains."
	Website      = "https://www.flomation.co"
	Icon         = "route+plus"
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
	{Name: "transit_gateway_id", Type: core.ConnectionTypeString, Label: "Transit Gateway ID", Placeholder: "tgw-0123456789abcdef0", Required: true},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "route_table", Type: core.ConnectionTypeObject, Label: "Route Table"},
	{Name: "transit_gateway_route_table_id", Type: core.ConnectionTypeString, Label: "Transit Gateway Route Table ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	tgwID := strings.TrimSpace(awscommon.InputString("transit_gateway_id", inputs))
	if tgwID == "" {
		return nil, fmt.Errorf("transit_gateway_id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.CreateTransitGatewayRouteTableInput{
		TransitGatewayId: aws.String(tgwID),
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.TagSpecifications = []ec2types.TagSpecification{{
			ResourceType: ec2types.ResourceTypeTransitGatewayRouteTable,
			Tags:         tags,
		}}
	}

	out, err := client.CreateTransitGatewayRouteTable(ctx, in)
	if err != nil {
		return nil, err
	}

	routeTable := map[string]interface{}{}
	id := ""
	if rt := out.TransitGatewayRouteTable; rt != nil {
		id = aws.ToString(rt.TransitGatewayRouteTableId)
		routeTable = map[string]interface{}{
			"transit_gateway_route_table_id":  id,
			"transit_gateway_id":              aws.ToString(rt.TransitGatewayId),
			"state":                           string(rt.State),
			"default_association_route_table": aws.ToBool(rt.DefaultAssociationRouteTable),
			"default_propagation_route_table": aws.ToBool(rt.DefaultPropagationRouteTable),
		}
	}

	return map[string]interface{}{
		"tool_result":                    fmt.Sprintf("Created transit gateway route table %s", id),
		"route_table":                    routeTable,
		"transit_gateway_route_table_id": id,
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
