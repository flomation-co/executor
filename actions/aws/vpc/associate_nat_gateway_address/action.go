// Package aws_vpc_associate_nat_gateway_address associates one or more Elastic
// IP allocations (secondary addresses) with a public NAT gateway.
package aws_vpc_associate_nat_gateway_address

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
	Name         = "AWS VPC Associate NAT Gateway Address"
	Description  = "Associate Elastic IP allocations (secondary addresses) with a public NAT gateway."
	Website      = "https://www.flomation.co"
	Icon         = "route+link"
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
	{Name: "nat_gateway_id", Type: core.ConnectionTypeString, Label: "NAT Gateway ID", Placeholder: "nat-0abc123", Required: true},
	{Name: "allocation_ids", Type: core.ConnectionTypeString, Label: "Elastic IP Allocation IDs", Placeholder: "Comma-separated, e.g. eipalloc-0abc,eipalloc-0def", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "nat_gateway", Type: core.ConnectionTypeObject, Label: "NAT Gateway"},
	{Name: "nat_gateway_id", Type: core.ConnectionTypeString, Label: "NAT Gateway ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	natID := strings.TrimSpace(awscommon.InputString("nat_gateway_id", inputs))
	if natID == "" {
		return nil, fmt.Errorf("nat_gateway_id is required")
	}
	allocIDs := awscommon.InputStrings("allocation_ids", inputs)
	if len(allocIDs) == 0 {
		return nil, fmt.Errorf("allocation_ids is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	out, err := client.AssociateNatGatewayAddress(ctx, &ec2.AssociateNatGatewayAddressInput{
		NatGatewayId:  aws.String(natID),
		AllocationIds: allocIDs,
	})
	if err != nil {
		return nil, err
	}

	var addresses []map[string]interface{}
	for _, a := range out.NatGatewayAddresses {
		addresses = append(addresses, map[string]interface{}{
			"allocation_id":        aws.ToString(a.AllocationId),
			"association_id":       aws.ToString(a.AssociationId),
			"public_ip":            aws.ToString(a.PublicIp),
			"private_ip":           aws.ToString(a.PrivateIp),
			"network_interface_id": aws.ToString(a.NetworkInterfaceId),
			"is_primary":           aws.ToBool(a.IsPrimary),
			"status":               string(a.Status),
		})
	}
	id := aws.ToString(out.NatGatewayId)
	if id == "" {
		id = natID
	}
	natGateway := map[string]interface{}{
		"nat_gateway_id":        id,
		"nat_gateway_addresses": addresses,
	}

	return map[string]interface{}{
		"tool_result":    fmt.Sprintf("Associated %d address(es) with NAT gateway %s", len(allocIDs), id),
		"nat_gateway":    natGateway,
		"nat_gateway_id": id,
	}, nil
}
