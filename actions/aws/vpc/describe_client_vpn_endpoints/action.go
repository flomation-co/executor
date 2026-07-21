// Package aws_vpc_describe_client_vpn_endpoints lists AWS Client VPN endpoints,
// optionally filtered by id or tags.
package aws_vpc_describe_client_vpn_endpoints

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
	Name         = "AWS VPC Describe Client VPN Endpoints"
	Description  = "List AWS Client VPN endpoints, optionally filtered by id or tags."
	Website      = "https://www.flomation.co"
	Icon         = "key+magnifying-glass"
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
	{Name: "client_vpn_endpoint_id", Type: core.ConnectionTypeString, Label: "Client VPN Endpoint ID (optional)", Placeholder: "Leave blank to list all"},
	{Name: "filter_tags", Type: core.ConnectionTypeKeyValueArray, Label: "Filter by Tags (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "endpoints", Type: core.ConnectionTypeObject, Label: "Client VPN Endpoints"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.DescribeClientVpnEndpointsInput{
		Filters: awscommon.BuildEC2Filters(inputs, nil),
	}
	if ids := awscommon.InputStrings("client_vpn_endpoint_id", inputs); len(ids) > 0 {
		in.ClientVpnEndpointIds = ids
	}

	var endpoints []map[string]interface{}
	paginator := ec2.NewDescribeClientVpnEndpointsPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for i := range page.ClientVpnEndpoints {
			e := &page.ClientVpnEndpoints[i]
			m := map[string]interface{}{
				"client_vpn_endpoint_id": aws.ToString(e.ClientVpnEndpointId),
				"description":            aws.ToString(e.Description),
				"client_cidr_block":      aws.ToString(e.ClientCidrBlock),
				"dns_name":               aws.ToString(e.DnsName),
				"server_certificate_arn": aws.ToString(e.ServerCertificateArn),
				"vpc_id":                 aws.ToString(e.VpcId),
				"transport_protocol":     string(e.TransportProtocol),
				"split_tunnel":           aws.ToBool(e.SplitTunnel),
				"creation_time":          aws.ToString(e.CreationTime),
			}
			if e.Status != nil {
				m["status"] = string(e.Status.Code)
				m["status_message"] = aws.ToString(e.Status.Message)
			}
			endpoints = append(endpoints, m)
		}
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d Client VPN endpoint(s)", len(endpoints)),
		"endpoints":   endpoints,
		"count":       len(endpoints),
	}, nil
}
