// Package aws_vpc_disassociate_nat_gateway_address disassociates one or more
// secondary Elastic IP addresses from a public NAT gateway.
package aws_vpc_disassociate_nat_gateway_address

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
	Name         = "AWS VPC Disassociate NAT Gateway Address"
	Description  = "Disassociate secondary Elastic IP addresses from a public NAT gateway."
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
	{Name: "nat_gateway_id", Type: core.ConnectionTypeString, Label: "NAT Gateway ID", Placeholder: "nat-0abc123", Required: true},
	{Name: "association_ids", Type: core.ConnectionTypeString, Label: "Address Association IDs", Placeholder: "Comma-separated, e.g. eipassoc-0abc,eipassoc-0def", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	natID := strings.TrimSpace(awscommon.InputString("nat_gateway_id", inputs))
	if natID == "" {
		return nil, fmt.Errorf("nat_gateway_id is required")
	}
	assocIDs := awscommon.InputStrings("association_ids", inputs)
	if len(assocIDs) == 0 {
		return nil, fmt.Errorf("association_ids is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	if _, err := client.DisassociateNatGatewayAddress(ctx, &ec2.DisassociateNatGatewayAddressInput{
		NatGatewayId:   aws.String(natID),
		AssociationIds: assocIDs,
	}); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Disassociated %d address(es) from NAT gateway %s", len(assocIDs), natID),
	}, nil
}
