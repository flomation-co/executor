// Package aws_vpc_allocate_ipam_pool_cidr allocates a CIDR from an IPAM pool.
package aws_vpc_allocate_ipam_pool_cidr

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
	Name         = "AWS IPAM Allocate Pool CIDR"
	Description  = "Allocate a CIDR from an IPAM pool by CIDR or netmask length."
	Website      = "https://www.flomation.co"
	Icon         = "layer-group+plus"
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
	{Name: "ipam_pool_id", Type: core.ConnectionTypeString, Label: "IPAM Pool ID", Required: true},
	{Name: "cidr", Type: core.ConnectionTypeString, Label: "CIDR (optional)", Placeholder: "10.0.0.0/24"},
	{Name: "netmask_length", Type: core.ConnectionTypeInteger, Label: "Netmask Length (optional)", Placeholder: "24"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "allocation", Type: core.ConnectionTypeObject, Label: "IPAM Pool Allocation"},
	{Name: "ipam_pool_allocation_id", Type: core.ConnectionTypeString, Label: "IPAM Pool Allocation ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	id := strings.TrimSpace(awscommon.InputString("ipam_pool_id", inputs))
	if id == "" {
		return nil, fmt.Errorf("ipam_pool_id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.AllocateIpamPoolCidrInput{IpamPoolId: aws.String(id)}
	if c := strings.TrimSpace(awscommon.InputString("cidr", inputs)); c != "" {
		in.Cidr = aws.String(c)
	}
	if n := core.FindConnection("netmask_length", inputs); n != nil {
		if v := n.Number(); v != nil {
			in.NetmaskLength = aws.Int32(int32(*v))
		}
	}
	if d := strings.TrimSpace(awscommon.InputString("description", inputs)); d != "" {
		in.Description = aws.String(d)
	}

	out, err := client.AllocateIpamPoolCidr(ctx, in)
	if err != nil {
		return nil, err
	}

	alloc := map[string]interface{}{}
	allocID := ""
	cidrStr := ""
	if out.IpamPoolAllocation != nil {
		allocID = aws.ToString(out.IpamPoolAllocation.IpamPoolAllocationId)
		cidrStr = aws.ToString(out.IpamPoolAllocation.Cidr)
		alloc = map[string]interface{}{
			"ipam_pool_allocation_id": allocID,
			"cidr":                    cidrStr,
			"resource_type":           string(out.IpamPoolAllocation.ResourceType),
			"resource_id":             aws.ToString(out.IpamPoolAllocation.ResourceId),
			"description":             aws.ToString(out.IpamPoolAllocation.Description),
		}
	}

	return map[string]interface{}{
		"tool_result":             fmt.Sprintf("Allocated CIDR %s from pool %s", cidrStr, id),
		"allocation":              alloc,
		"ipam_pool_allocation_id": allocID,
	}, nil
}
