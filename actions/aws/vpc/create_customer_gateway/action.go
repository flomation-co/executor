// Package aws_vpc_create_customer_gateway registers a customer gateway (the
// on-premises end of a Site-to-Site VPN).
package aws_vpc_create_customer_gateway

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
	Name         = "AWS VPC Create Customer Gateway"
	Description  = "Register a customer gateway (the on-premises end of a Site-to-Site VPN)."
	Website      = "https://www.flomation.co"
	Icon         = "lock+plus"
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
	{Name: "type", Type: core.ConnectionTypeString, Label: "VPN Type", Required: true, Options: []core.ConnectionOption{
		{Name: "ipsec.1", Value: "ipsec.1"},
	}},
	{Name: "bgp_asn", Type: core.ConnectionTypeInteger, Label: "BGP ASN (optional)", Placeholder: "65000"},
	{Name: "public_ip", Type: core.ConnectionTypeString, Label: "Outside IP Address (optional)", Placeholder: "203.0.113.10"},
	{Name: "certificate_arn", Type: core.ConnectionTypeString, Label: "Certificate ARN (optional)"},
	{Name: "device_name", Type: core.ConnectionTypeString, Label: "Device Name (optional)"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "customer_gateway", Type: core.ConnectionTypeObject, Label: "Customer Gateway"},
	{Name: "customer_gateway_id", Type: core.ConnectionTypeString, Label: "Customer Gateway ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	vpnType := strings.TrimSpace(awscommon.InputString("type", inputs))
	if vpnType == "" {
		return nil, fmt.Errorf("type is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.CreateCustomerGatewayInput{Type: ec2types.GatewayType(vpnType)}
	if n, ok := awscommon.InputInt("bgp_asn", inputs); ok {
		in.BgpAsn = aws.Int32(int32(n))
	}
	if ip := strings.TrimSpace(awscommon.InputString("public_ip", inputs)); ip != "" {
		in.IpAddress = aws.String(ip)
	}
	if arn := strings.TrimSpace(awscommon.InputString("certificate_arn", inputs)); arn != "" {
		in.CertificateArn = aws.String(arn)
	}
	if dn := strings.TrimSpace(awscommon.InputString("device_name", inputs)); dn != "" {
		in.DeviceName = aws.String(dn)
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.TagSpecifications = []ec2types.TagSpecification{{
			ResourceType: ec2types.ResourceTypeCustomerGateway,
			Tags:         tags,
		}}
	}

	out, err := client.CreateCustomerGateway(ctx, in)
	if err != nil {
		return nil, err
	}

	cg := map[string]interface{}{}
	id := ""
	if out.CustomerGateway != nil {
		id = aws.ToString(out.CustomerGateway.CustomerGatewayId)
		cg = map[string]interface{}{
			"customer_gateway_id": id,
			"state":               aws.ToString(out.CustomerGateway.State),
			"type":                aws.ToString(out.CustomerGateway.Type),
			"ip_address":          aws.ToString(out.CustomerGateway.IpAddress),
			"bgp_asn":             aws.ToString(out.CustomerGateway.BgpAsn),
			"device_name":         aws.ToString(out.CustomerGateway.DeviceName),
		}
	}

	return map[string]interface{}{
		"tool_result":         fmt.Sprintf("Created customer gateway %s", id),
		"customer_gateway":    cg,
		"customer_gateway_id": id,
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
