// Package aws_vpc_modify_transit_gateway updates a transit gateway's settings.
package aws_vpc_modify_transit_gateway

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
	Name         = "AWS VPC Modify Transit Gateway"
	Description  = "Update a transit gateway's description."
	Website      = "https://www.flomation.co"
	Icon         = "route+pen"
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
	{Name: "transit_gateway_id", Type: core.ConnectionTypeString, Label: "Transit Gateway ID", Placeholder: "tgw-0abc", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description (optional)", Placeholder: "New description"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "transit_gateway", Type: core.ConnectionTypeObject, Label: "Transit Gateway"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	id := strings.TrimSpace(awscommon.InputString("transit_gateway_id", inputs))
	if id == "" {
		return nil, fmt.Errorf("transit_gateway_id is required")
	}

	in := &ec2.ModifyTransitGatewayInput{TransitGatewayId: aws.String(id)}
	changed := false
	if d := strings.TrimSpace(awscommon.InputString("description", inputs)); d != "" {
		in.Description = aws.String(d)
		changed = true
	}
	if !changed {
		return nil, fmt.Errorf("provide at least one change (description)")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	out, err := client.ModifyTransitGateway(ctx, in)
	if err != nil {
		return nil, err
	}

	tgw := map[string]interface{}{}
	if out.TransitGateway != nil {
		g := out.TransitGateway
		tgw = map[string]interface{}{
			"transit_gateway_id":  aws.ToString(g.TransitGatewayId),
			"transit_gateway_arn": aws.ToString(g.TransitGatewayArn),
			"state":               string(g.State),
			"description":         aws.ToString(g.Description),
		}
	}

	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Modified transit gateway %s", id),
		"transit_gateway": tgw,
	}, nil
}
