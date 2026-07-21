// Package aws_vpc_create_vpc_endpoint_service_configuration creates a VPC
// endpoint service configuration (the provider side of PrivateLink).
package aws_vpc_create_vpc_endpoint_service_configuration

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
	Name         = "AWS VPC Create Endpoint Service Configuration"
	Description  = "Create a VPC endpoint service (PrivateLink provider) fronted by load balancers."
	Website      = "https://www.flomation.co"
	Icon         = "link+plus"
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
	{Name: "network_load_balancer_arns", Type: core.ConnectionTypeString, Label: "Network Load Balancer ARNs (optional)", Placeholder: "Comma-separated NLB ARNs"},
	{Name: "gateway_load_balancer_arns", Type: core.ConnectionTypeString, Label: "Gateway Load Balancer ARNs (optional)", Placeholder: "Comma-separated GWLB ARNs"},
	{Name: "acceptance_required", Type: core.ConnectionTypeBoolean, Label: "Require Acceptance (optional)"},
	{Name: "private_dns_name", Type: core.ConnectionTypeString, Label: "Private DNS Name (optional)", Placeholder: "service.example.com"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "service", Type: core.ConnectionTypeObject, Label: "Service Configuration"},
	{Name: "service_id", Type: core.ConnectionTypeString, Label: "Service ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	nlbArns := awscommon.InputStrings("network_load_balancer_arns", inputs)
	gwlbArns := awscommon.InputStrings("gateway_load_balancer_arns", inputs)
	if len(nlbArns) == 0 && len(gwlbArns) == 0 {
		return nil, fmt.Errorf("at least one network or gateway load balancer ARN is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.CreateVpcEndpointServiceConfigurationInput{}
	if len(nlbArns) > 0 {
		in.NetworkLoadBalancerArns = nlbArns
	}
	if len(gwlbArns) > 0 {
		in.GatewayLoadBalancerArns = gwlbArns
	}
	if c := core.FindConnection("acceptance_required", inputs); c != nil {
		if b := c.Boolean(); b != nil {
			in.AcceptanceRequired = aws.Bool(*b)
		}
	}
	if v := strings.TrimSpace(awscommon.InputString("private_dns_name", inputs)); v != "" {
		in.PrivateDnsName = aws.String(v)
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.TagSpecifications = []ec2types.TagSpecification{{
			ResourceType: ec2types.ResourceTypeVpcEndpointService,
			Tags:         tags,
		}}
	}

	out, err := client.CreateVpcEndpointServiceConfiguration(ctx, in)
	if err != nil {
		return nil, err
	}

	svc := map[string]interface{}{}
	id := ""
	if out.ServiceConfiguration != nil {
		s := out.ServiceConfiguration
		id = aws.ToString(s.ServiceId)
		svc = map[string]interface{}{
			"service_id":                 id,
			"service_name":               aws.ToString(s.ServiceName),
			"service_state":              string(s.ServiceState),
			"acceptance_required":        aws.ToBool(s.AcceptanceRequired),
			"private_dns_name":           aws.ToString(s.PrivateDnsName),
			"availability_zones":         s.AvailabilityZones,
			"base_endpoint_dns_names":    s.BaseEndpointDnsNames,
			"network_load_balancer_arns": s.NetworkLoadBalancerArns,
			"gateway_load_balancer_arns": s.GatewayLoadBalancerArns,
			"tags":                       flattenTags(s.Tags),
		}
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created VPC endpoint service configuration %s", id),
		"service":     svc,
		"service_id":  id,
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

func flattenTags(tags []ec2types.Tag) map[string]string {
	out := map[string]string{}
	for _, t := range tags {
		out[aws.ToString(t.Key)] = aws.ToString(t.Value)
	}
	return out
}
