// Package aws_vpc_create_ipam_scope creates an AWS IPAM scope.
package aws_vpc_create_ipam_scope

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
	Name         = "AWS IPAM Create Scope"
	Description  = "Create a private IPAM scope within an AWS IPAM."
	Website      = "https://www.flomation.co"
	Icon         = "object-group+plus"
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
	{Name: "ipam_id", Type: core.ConnectionTypeString, Label: "IPAM ID", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description (optional)"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "ipam_scope", Type: core.ConnectionTypeObject, Label: "IPAM Scope"},
	{Name: "ipam_scope_id", Type: core.ConnectionTypeString, Label: "IPAM Scope ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	ipamID := strings.TrimSpace(awscommon.InputString("ipam_id", inputs))
	if ipamID == "" {
		return nil, fmt.Errorf("ipam_id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.CreateIpamScopeInput{IpamId: aws.String(ipamID)}
	if d := strings.TrimSpace(awscommon.InputString("description", inputs)); d != "" {
		in.Description = aws.String(d)
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.TagSpecifications = []ec2types.TagSpecification{{
			ResourceType: ec2types.ResourceTypeIpamScope,
			Tags:         tags,
		}}
	}

	out, err := client.CreateIpamScope(ctx, in)
	if err != nil {
		return nil, err
	}

	scope := map[string]interface{}{}
	id := ""
	if out.IpamScope != nil {
		id = aws.ToString(out.IpamScope.IpamScopeId)
		scope = map[string]interface{}{
			"ipam_scope_id":   id,
			"ipam_scope_arn":  aws.ToString(out.IpamScope.IpamScopeArn),
			"ipam_scope_type": string(out.IpamScope.IpamScopeType),
			"is_default":      aws.ToBool(out.IpamScope.IsDefault),
			"state":           string(out.IpamScope.State),
			"description":     aws.ToString(out.IpamScope.Description),
		}
	}

	return map[string]interface{}{
		"tool_result":   fmt.Sprintf("Created IPAM scope %s", id),
		"ipam_scope":    scope,
		"ipam_scope_id": id,
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
