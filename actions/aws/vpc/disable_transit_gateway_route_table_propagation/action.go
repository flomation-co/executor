// Package aws_vpc_disable_transit_gateway_route_table_propagation disables route propagation for an attachment.
package aws_vpc_disable_transit_gateway_route_table_propagation

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
	Name         = "AWS VPC Disable Transit Gateway Route Table Propagation"
	Description  = "Disable route propagation from an attachment into a route table."
	Website      = "https://www.flomation.co"
	Icon         = "route+minus"
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
	{Name: "transit_gateway_route_table_id", Type: core.ConnectionTypeString, Label: "Transit Gateway Route Table ID", Placeholder: "tgw-rtb-0123456789abcdef0", Required: true},
	{Name: "transit_gateway_attachment_id", Type: core.ConnectionTypeString, Label: "Transit Gateway Attachment ID", Placeholder: "tgw-attach-0123456789abcdef0", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "propagation", Type: core.ConnectionTypeObject, Label: "Propagation"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	rtID := strings.TrimSpace(awscommon.InputString("transit_gateway_route_table_id", inputs))
	if rtID == "" {
		return nil, fmt.Errorf("transit_gateway_route_table_id is required")
	}
	attID := strings.TrimSpace(awscommon.InputString("transit_gateway_attachment_id", inputs))
	if attID == "" {
		return nil, fmt.Errorf("transit_gateway_attachment_id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	out, err := client.DisableTransitGatewayRouteTablePropagation(ctx, &ec2.DisableTransitGatewayRouteTablePropagationInput{
		TransitGatewayRouteTableId: aws.String(rtID),
		TransitGatewayAttachmentId: aws.String(attID),
	})
	if err != nil {
		return nil, err
	}

	propagation := map[string]interface{}{}
	if p := out.Propagation; p != nil {
		propagation = map[string]interface{}{
			"transit_gateway_route_table_id": aws.ToString(p.TransitGatewayRouteTableId),
			"transit_gateway_attachment_id":  aws.ToString(p.TransitGatewayAttachmentId),
			"resource_id":                    aws.ToString(p.ResourceId),
			"resource_type":                  string(p.ResourceType),
			"state":                          string(p.State),
		}
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Disabled propagation from attachment %s into route table %s", attID, rtID),
		"propagation": propagation,
	}, nil
}
