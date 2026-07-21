// Package aws_vpc_modify_transit_gateway_vpc_attachment adds or removes subnets on a VPC attachment.
package aws_vpc_modify_transit_gateway_vpc_attachment

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
	Name         = "AWS VPC Modify Transit Gateway VPC Attachment"
	Description  = "Add or remove subnets on a transit gateway VPC attachment."
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
	{Name: "transit_gateway_attachment_id", Type: core.ConnectionTypeString, Label: "Transit Gateway Attachment ID", Placeholder: "tgw-attach-0123456789abcdef0", Required: true},
	{Name: "add_subnet_ids", Type: core.ConnectionTypeString, Label: "Subnet IDs to Add (optional)", Placeholder: "Comma-separated; one subnet per Availability Zone"},
	{Name: "remove_subnet_ids", Type: core.ConnectionTypeString, Label: "Subnet IDs to Remove (optional)", Placeholder: "Comma-separated"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "attachment", Type: core.ConnectionTypeObject, Label: "Attachment"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	id := strings.TrimSpace(awscommon.InputString("transit_gateway_attachment_id", inputs))
	if id == "" {
		return nil, fmt.Errorf("transit_gateway_attachment_id is required")
	}
	add := awscommon.InputStrings("add_subnet_ids", inputs)
	remove := awscommon.InputStrings("remove_subnet_ids", inputs)
	if len(add) == 0 && len(remove) == 0 {
		return nil, fmt.Errorf("provide at least one subnet to add or remove")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.ModifyTransitGatewayVpcAttachmentInput{
		TransitGatewayAttachmentId: aws.String(id),
	}
	if len(add) > 0 {
		in.AddSubnetIds = add
	}
	if len(remove) > 0 {
		in.RemoveSubnetIds = remove
	}

	out, err := client.ModifyTransitGatewayVpcAttachment(ctx, in)
	if err != nil {
		return nil, err
	}

	attachment := map[string]interface{}{}
	if a := out.TransitGatewayVpcAttachment; a != nil {
		attachment = map[string]interface{}{
			"transit_gateway_attachment_id": aws.ToString(a.TransitGatewayAttachmentId),
			"transit_gateway_id":            aws.ToString(a.TransitGatewayId),
			"vpc_id":                        aws.ToString(a.VpcId),
			"state":                         string(a.State),
			"subnet_ids":                    a.SubnetIds,
		}
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Modified transit gateway VPC attachment %s", id),
		"attachment":  attachment,
	}, nil
}
