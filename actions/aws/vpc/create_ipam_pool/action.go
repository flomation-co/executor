// Package aws_vpc_create_ipam_pool creates an AWS IPAM pool.
package aws_vpc_create_ipam_pool

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
	Name         = "AWS IPAM Create Pool"
	Description  = "Create an IPAM pool within a scope for allocating CIDRs."
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
	{Name: "ipam_scope_id", Type: core.ConnectionTypeString, Label: "IPAM Scope ID", Required: true},
	{Name: "address_family", Type: core.ConnectionTypeString, Label: "Address Family", Required: true, Options: []core.ConnectionOption{
		{Name: "IPv4", Value: "ipv4"},
		{Name: "IPv6", Value: "ipv6"},
	}},
	{Name: "source_ipam_pool_id", Type: core.ConnectionTypeString, Label: "Source IPAM Pool ID (optional)"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description (optional)"},
	{Name: "auto_import", Type: core.ConnectionTypeBoolean, Label: "Auto Import"},
	{Name: "locale", Type: core.ConnectionTypeString, Label: "Locale (optional)", Placeholder: "eu-west-2"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "ipam_pool", Type: core.ConnectionTypeObject, Label: "IPAM Pool"},
	{Name: "ipam_pool_id", Type: core.ConnectionTypeString, Label: "IPAM Pool ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	scopeID := strings.TrimSpace(awscommon.InputString("ipam_scope_id", inputs))
	if scopeID == "" {
		return nil, fmt.Errorf("ipam_scope_id is required")
	}
	family := strings.TrimSpace(awscommon.InputString("address_family", inputs))
	if family == "" {
		return nil, fmt.Errorf("address_family is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.CreateIpamPoolInput{
		IpamScopeId:   aws.String(scopeID),
		AddressFamily: ec2types.AddressFamily(family),
	}
	if s := strings.TrimSpace(awscommon.InputString("source_ipam_pool_id", inputs)); s != "" {
		in.SourceIpamPoolId = aws.String(s)
	}
	if d := strings.TrimSpace(awscommon.InputString("description", inputs)); d != "" {
		in.Description = aws.String(d)
	}
	if c := core.FindConnection("auto_import", inputs); c != nil {
		if b := c.Boolean(); b != nil {
			in.AutoImport = aws.Bool(*b)
		}
	}
	if l := strings.TrimSpace(awscommon.InputString("locale", inputs)); l != "" {
		in.Locale = aws.String(l)
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.TagSpecifications = []ec2types.TagSpecification{{
			ResourceType: ec2types.ResourceTypeIpamPool,
			Tags:         tags,
		}}
	}

	out, err := client.CreateIpamPool(ctx, in)
	if err != nil {
		return nil, err
	}

	pool := map[string]interface{}{}
	id := ""
	if out.IpamPool != nil {
		id = aws.ToString(out.IpamPool.IpamPoolId)
		pool = map[string]interface{}{
			"ipam_pool_id":   id,
			"ipam_pool_arn":  aws.ToString(out.IpamPool.IpamPoolArn),
			"ipam_scope_arn": aws.ToString(out.IpamPool.IpamScopeArn),
			"address_family": string(out.IpamPool.AddressFamily),
			"state":          string(out.IpamPool.State),
			"locale":         aws.ToString(out.IpamPool.Locale),
			"description":    aws.ToString(out.IpamPool.Description),
		}
	}

	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Created IPAM pool %s", id),
		"ipam_pool":    pool,
		"ipam_pool_id": id,
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
