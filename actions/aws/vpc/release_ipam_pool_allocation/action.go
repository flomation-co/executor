// Package aws_vpc_release_ipam_pool_allocation releases an IPAM pool allocation.
package aws_vpc_release_ipam_pool_allocation

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
	Name         = "AWS IPAM Release Pool Allocation"
	Description  = "Release a CIDR allocation back to an IPAM pool."
	Website      = "https://www.flomation.co"
	Icon         = "layer-group+trash"
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
	{Name: "ipam_pool_allocation_id", Type: core.ConnectionTypeString, Label: "IPAM Pool Allocation ID", Required: true},
	{Name: "cidr", Type: core.ConnectionTypeString, Label: "CIDR", Required: true, Placeholder: "10.0.0.0/24"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	poolID := strings.TrimSpace(awscommon.InputString("ipam_pool_id", inputs))
	if poolID == "" {
		return nil, fmt.Errorf("ipam_pool_id is required")
	}
	allocID := strings.TrimSpace(awscommon.InputString("ipam_pool_allocation_id", inputs))
	if allocID == "" {
		return nil, fmt.Errorf("ipam_pool_allocation_id is required")
	}
	cidr := strings.TrimSpace(awscommon.InputString("cidr", inputs))
	if cidr == "" {
		return nil, fmt.Errorf("cidr is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	out, err := client.ReleaseIpamPoolAllocation(ctx, &ec2.ReleaseIpamPoolAllocationInput{
		IpamPoolId:           aws.String(poolID),
		IpamPoolAllocationId: aws.String(allocID),
		Cidr:                 aws.String(cidr),
	})
	if err != nil {
		return nil, err
	}

	success := aws.ToBool(out.Success)

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Released allocation %s (%s) from pool %s", allocID, cidr, poolID),
		"success":     success,
	}, nil
}
