// Package aws_vpc_modify_vpc_peering_connection_options changes the DNS
// resolution options of a VPC peering connection (requester and/or accepter).
package aws_vpc_modify_vpc_peering_connection_options

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
	Name         = "AWS VPC Modify Peering Connection Options"
	Description  = "Change the DNS resolution options of a VPC peering connection (requester/accepter)."
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
	{Name: "vpc_peering_connection_id", Type: core.ConnectionTypeString, Label: "VPC Peering Connection ID", Placeholder: "pcx-0abc123", Required: true},
	{Name: "requester_allow_dns_resolution", Type: core.ConnectionTypeBoolean, Label: "Requester: Allow DNS Resolution from Remote VPC (optional)"},
	{Name: "accepter_allow_dns_resolution", Type: core.ConnectionTypeBoolean, Label: "Accepter: Allow DNS Resolution from Remote VPC (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	id := strings.TrimSpace(awscommon.InputString("vpc_peering_connection_id", inputs))
	if id == "" {
		return nil, fmt.Errorf("vpc_peering_connection_id is required")
	}

	in := &ec2.ModifyVpcPeeringConnectionOptionsInput{VpcPeeringConnectionId: aws.String(id)}
	changed := false
	if b := boolPtr("requester_allow_dns_resolution", inputs); b != nil {
		in.RequesterPeeringConnectionOptions = &ec2types.PeeringConnectionOptionsRequest{
			AllowDnsResolutionFromRemoteVpc: b,
		}
		changed = true
	}
	if b := boolPtr("accepter_allow_dns_resolution", inputs); b != nil {
		in.AccepterPeeringConnectionOptions = &ec2types.PeeringConnectionOptionsRequest{
			AllowDnsResolutionFromRemoteVpc: b,
		}
		changed = true
	}
	if !changed {
		return nil, fmt.Errorf("provide at least one of requester_allow_dns_resolution or accepter_allow_dns_resolution")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	if _, err := client.ModifyVpcPeeringConnectionOptions(ctx, in); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Modified peering connection options for %s", id),
	}, nil
}

// boolPtr returns a nilable bool for a tri-state boolean input: nil when the
// input is absent/unset so the AWS field is left untouched.
func boolPtr(name string, inputs []*core.Connection) *bool {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return nil
	}
	if b := c.Boolean(); b != nil {
		return aws.Bool(*b)
	}
	return nil
}
