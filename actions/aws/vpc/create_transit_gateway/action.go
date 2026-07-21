// Package aws_vpc_create_transit_gateway creates a transit gateway.
package aws_vpc_create_transit_gateway

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
	Name         = "AWS VPC Create Transit Gateway"
	Description  = "Create a transit gateway to connect VPCs and on-premises networks."
	Website      = "https://www.flomation.co"
	Icon         = "route+plus"
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
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description (optional)", Placeholder: "e.g. Production transit gateway"},
	{Name: "amazon_side_asn", Type: core.ConnectionTypeInteger, Label: "Amazon Side ASN (optional)", Placeholder: "64512–65534, or 4200000000–4294967294"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "transit_gateway", Type: core.ConnectionTypeObject, Label: "Transit Gateway"},
	{Name: "transit_gateway_id", Type: core.ConnectionTypeString, Label: "Transit Gateway ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.CreateTransitGatewayInput{}
	if d := strings.TrimSpace(awscommon.InputString("description", inputs)); d != "" {
		in.Description = aws.String(d)
	}
	if asn, ok := awscommon.InputInt("amazon_side_asn", inputs); ok {
		in.Options = &ec2types.TransitGatewayRequestOptions{AmazonSideAsn: aws.Int64(asn)}
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.TagSpecifications = []ec2types.TagSpecification{{
			ResourceType: ec2types.ResourceTypeTransitGateway,
			Tags:         tags,
		}}
	}

	out, err := client.CreateTransitGateway(ctx, in)
	if err != nil {
		return nil, err
	}

	tgw := map[string]interface{}{}
	id := ""
	if out.TransitGateway != nil {
		g := out.TransitGateway
		id = aws.ToString(g.TransitGatewayId)
		tgw = map[string]interface{}{
			"transit_gateway_id":  id,
			"transit_gateway_arn": aws.ToString(g.TransitGatewayArn),
			"state":               string(g.State),
			"owner_id":            aws.ToString(g.OwnerId),
			"description":         aws.ToString(g.Description),
		}
	}

	return map[string]interface{}{
		"tool_result":        fmt.Sprintf("Created transit gateway %s", id),
		"transit_gateway":    tgw,
		"transit_gateway_id": id,
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
