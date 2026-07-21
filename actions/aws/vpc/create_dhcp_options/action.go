// Package aws_vpc_create_dhcp_options creates a DHCP options set.
package aws_vpc_create_dhcp_options

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
	Name         = "AWS VPC Create DHCP Options"
	Description  = "Create a DHCP options set with domain name, DNS, and NTP servers."
	Website      = "https://www.flomation.co"
	Icon         = "list+plus"
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
	{Name: "domain_name", Type: core.ConnectionTypeString, Label: "Domain Name (optional)", Placeholder: "example.internal"},
	{Name: "domain_name_servers", Type: core.ConnectionTypeString, Label: "DNS Servers (optional)", Placeholder: "Comma-separated, e.g. 10.0.0.2,AmazonProvidedDNS"},
	{Name: "ntp_servers", Type: core.ConnectionTypeString, Label: "NTP Servers (optional)", Placeholder: "Comma-separated, e.g. 169.254.169.123"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "dhcp_options", Type: core.ConnectionTypeObject, Label: "DHCP Options"},
	{Name: "dhcp_options_id", Type: core.ConnectionTypeString, Label: "DHCP Options ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	var configs []ec2types.NewDhcpConfiguration
	if v := strings.TrimSpace(awscommon.InputString("domain_name", inputs)); v != "" {
		configs = append(configs, ec2types.NewDhcpConfiguration{Key: aws.String("domain-name"), Values: []string{v}})
	}
	if v := awscommon.InputStrings("domain_name_servers", inputs); len(v) > 0 {
		configs = append(configs, ec2types.NewDhcpConfiguration{Key: aws.String("domain-name-servers"), Values: v})
	}
	if v := awscommon.InputStrings("ntp_servers", inputs); len(v) > 0 {
		configs = append(configs, ec2types.NewDhcpConfiguration{Key: aws.String("ntp-servers"), Values: v})
	}
	if len(configs) == 0 {
		return nil, fmt.Errorf("provide at least one of domain name, DNS servers, or NTP servers")
	}

	in := &ec2.CreateDhcpOptionsInput{DhcpConfigurations: configs}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.TagSpecifications = []ec2types.TagSpecification{{
			ResourceType: ec2types.ResourceTypeDhcpOptions,
			Tags:         tags,
		}}
	}

	out, err := client.CreateDhcpOptions(ctx, in)
	if err != nil {
		return nil, err
	}

	opts := map[string]interface{}{}
	id := ""
	if out.DhcpOptions != nil {
		id = aws.ToString(out.DhcpOptions.DhcpOptionsId)
		opts = map[string]interface{}{
			"dhcp_options_id": id,
			"owner_id":        aws.ToString(out.DhcpOptions.OwnerId),
		}
	}

	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Created DHCP options set %s", id),
		"dhcp_options":    opts,
		"dhcp_options_id": id,
	}, nil
}

func buildTags(inputs []*core.Connection) []ec2types.Tag {
	conn := core.FindConnection("tags", inputs)
	if conn == nil {
		return nil
	}
	var tags []ec2types.Tag
	for _, kv := range conn.KeyValuePairs() {
		k := strings.TrimSpace(kv.Key)
		if k == "" {
			continue
		}
		tags = append(tags, ec2types.Tag{Key: aws.String(k), Value: aws.String(kv.Value)})
	}
	return tags
}
