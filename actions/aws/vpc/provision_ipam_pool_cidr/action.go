// Package aws_vpc_provision_ipam_pool_cidr provisions a CIDR to an IPAM pool.
package aws_vpc_provision_ipam_pool_cidr

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
	Name         = "AWS IPAM Provision Pool CIDR"
	Description  = "Provision a CIDR to an IPAM pool by CIDR or netmask length."
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
	{Name: "cidr", Type: core.ConnectionTypeString, Label: "CIDR (optional)", Placeholder: "10.0.0.0/16"},
	{Name: "netmask_length", Type: core.ConnectionTypeInteger, Label: "Netmask Length (optional)", Placeholder: "16"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "cidr", Type: core.ConnectionTypeObject, Label: "IPAM Pool CIDR"},
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

	in := &ec2.ProvisionIpamPoolCidrInput{IpamPoolId: aws.String(id)}
	if c := strings.TrimSpace(awscommon.InputString("cidr", inputs)); c != "" {
		in.Cidr = aws.String(c)
	}
	if n := core.FindConnection("netmask_length", inputs); n != nil {
		if v := n.Number(); v != nil {
			in.NetmaskLength = aws.Int32(int32(*v))
		}
	}

	out, err := client.ProvisionIpamPoolCidr(ctx, in)
	if err != nil {
		return nil, err
	}

	cidr := map[string]interface{}{}
	cidrStr := ""
	if out.IpamPoolCidr != nil {
		cidrStr = aws.ToString(out.IpamPoolCidr.Cidr)
		cidr = map[string]interface{}{
			"ipam_pool_cidr_id": aws.ToString(out.IpamPoolCidr.IpamPoolCidrId),
			"cidr":              cidrStr,
			"state":             string(out.IpamPoolCidr.State),
		}
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Provisioned CIDR %s to pool %s", cidrStr, id),
		"cidr":        cidr,
	}, nil
}
