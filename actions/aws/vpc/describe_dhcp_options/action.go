// Package aws_vpc_describe_dhcp_options lists DHCP option sets, optionally
// narrowed by ID or tags.
package aws_vpc_describe_dhcp_options

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
	Name         = "AWS VPC Describe DHCP Options"
	Description  = "List DHCP option sets, optionally filtered by ID or tags."
	Website      = "https://www.flomation.co"
	Icon         = "list+magnifying-glass"
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
	{Name: "dhcp_options_id", Type: core.ConnectionTypeString, Label: "DHCP Options ID (optional)", Placeholder: "Leave blank to list all"},
	{Name: "filter_tags", Type: core.ConnectionTypeKeyValueArray, Label: "Filter by Tags (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "dhcp_option_sets", Type: core.ConnectionTypeObject, Label: "DHCP Option Sets"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.DescribeDhcpOptionsInput{
		Filters: awscommon.BuildEC2Filters(inputs, nil),
	}
	if id := awscommon.InputString("dhcp_options_id", inputs); id != "" {
		in.DhcpOptionsIds = []string{id}
	}

	var sets []map[string]interface{}
	paginator := ec2.NewDescribeDhcpOptionsPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for i := range page.DhcpOptions {
			d := &page.DhcpOptions[i]
			configs := make([]map[string]interface{}, 0, len(d.DhcpConfigurations))
			for j := range d.DhcpConfigurations {
				c := &d.DhcpConfigurations[j]
				values := make([]string, 0, len(c.Values))
				for k := range c.Values {
					values = append(values, aws.ToString(c.Values[k].Value))
				}
				configs = append(configs, map[string]interface{}{
					"key":    aws.ToString(c.Key),
					"values": values,
				})
			}
			sets = append(sets, map[string]interface{}{
				"dhcp_options_id": aws.ToString(d.DhcpOptionsId),
				"owner_id":        aws.ToString(d.OwnerId),
				"configurations":  configs,
				"tags":            flattenTags(d.Tags),
			})
		}
	}

	return map[string]interface{}{
		"tool_result":      fmt.Sprintf("Found %d DHCP option set(s)", len(sets)),
		"dhcp_option_sets": sets,
		"count":            len(sets),
	}, nil
}

func flattenTags(tags []ec2types.Tag) map[string]string {
	out := map[string]string{}
	for _, t := range tags {
		out[aws.ToString(t.Key)] = aws.ToString(t.Value)
	}
	return out
}
