// Package aws_vpc_describe_network_acls lists network ACLs, optionally narrowed
// by ID, VPC, or tags.
package aws_vpc_describe_network_acls

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS VPC Describe Network ACLs"
	Description  = "List network ACLs, optionally filtered by ID, VPC, or tags."
	Website      = "https://www.flomation.co"
	Icon         = "lock+magnifying-glass"
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
	{Name: "network_acl_id", Type: core.ConnectionTypeString, Label: "Network ACL ID (optional)", Placeholder: "Leave blank to list all"},
	{Name: "vpc_id", Type: core.ConnectionTypeString, Label: "VPC ID (optional filter)", Placeholder: "vpc-0abc"},
	{Name: "filter_tags", Type: core.ConnectionTypeKeyValueArray, Label: "Filter by Tags (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "network_acls", Type: core.ConnectionTypeObject, Label: "Network ACLs"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.DescribeNetworkAclsInput{
		Filters: awscommon.BuildEC2Filters(inputs, []awscommon.FilterSpec{
			{Input: "vpc_id", Filter: "vpc-id"},
		}),
	}
	if id := awscommon.InputString("network_acl_id", inputs); id != "" {
		in.NetworkAclIds = []string{id}
	}

	var acls []map[string]interface{}
	paginator := ec2.NewDescribeNetworkAclsPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for i := range page.NetworkAcls {
			a := &page.NetworkAcls[i]
			entries := make([]map[string]interface{}, 0, len(a.Entries))
			for j := range a.Entries {
				e := &a.Entries[j]
				entries = append(entries, map[string]interface{}{
					"rule_number": aws.ToInt32(e.RuleNumber),
					"protocol":    aws.ToString(e.Protocol),
					"rule_action": string(e.RuleAction),
					"egress":      aws.ToBool(e.Egress),
					"cidr_block":  aws.ToString(e.CidrBlock),
				})
			}
			acls = append(acls, map[string]interface{}{
				"network_acl_id": aws.ToString(a.NetworkAclId),
				"vpc_id":         aws.ToString(a.VpcId),
				"is_default":     aws.ToBool(a.IsDefault),
				"owner_id":       aws.ToString(a.OwnerId),
				"entries":        entries,
				"tags":           flattenTags(a.Tags),
			})
		}
	}

	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Found %d network ACL(s)", len(acls)),
		"network_acls": acls,
		"count":        len(acls),
	}, nil
}

func flattenTags(tags []ec2types.Tag) map[string]string {
	out := map[string]string{}
	for _, t := range tags {
		out[aws.ToString(t.Key)] = aws.ToString(t.Value)
	}
	return out
}
