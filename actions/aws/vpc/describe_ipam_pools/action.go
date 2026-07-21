// Package aws_vpc_describe_ipam_pools lists AWS IPAM pools.
package aws_vpc_describe_ipam_pools

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS IPAM Describe Pools"
	Description  = "List AWS IPAM pools, optionally filtered by pool id."
	Website      = "https://www.flomation.co"
	Icon         = "layer-group+magnifying-glass"
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
	{Name: "ipam_pool_id", Type: core.ConnectionTypeString, Label: "IPAM Pool ID (optional)", Placeholder: "Leave blank to list all"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "ipam_pools", Type: core.ConnectionTypeObject, Label: "IPAM Pools"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.DescribeIpamPoolsInput{}
	if id := awscommon.InputString("ipam_pool_id", inputs); id != "" {
		in.IpamPoolIds = []string{id}
	}

	var pools []map[string]interface{}
	paginator := ec2.NewDescribeIpamPoolsPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for i := range page.IpamPools {
			v := &page.IpamPools[i]
			pools = append(pools, map[string]interface{}{
				"ipam_pool_id":   aws.ToString(v.IpamPoolId),
				"ipam_pool_arn":  aws.ToString(v.IpamPoolArn),
				"ipam_scope_arn": aws.ToString(v.IpamScopeArn),
				"address_family": string(v.AddressFamily),
				"state":          string(v.State),
				"locale":         aws.ToString(v.Locale),
				"description":    aws.ToString(v.Description),
				"pool_depth":     aws.ToInt32(v.PoolDepth),
			})
		}
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d IPAM pool(s)", len(pools)),
		"ipam_pools":  pools,
		"count":       len(pools),
	}, nil
}
