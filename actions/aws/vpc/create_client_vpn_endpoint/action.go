// Package aws_vpc_create_client_vpn_endpoint creates an AWS Client VPN endpoint
// (the remote-user VPN product) for clients to establish VPN sessions.
package aws_vpc_create_client_vpn_endpoint

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
	Name         = "AWS VPC Create Client VPN Endpoint"
	Description  = "Create an AWS Client VPN endpoint for remote users to connect via a VPN client."
	Website      = "https://www.flomation.co"
	Icon         = "key+plus"
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
	{Name: "client_cidr_block", Type: core.ConnectionTypeString, Label: "Client CIDR Block", Placeholder: "10.0.0.0/22 (min /22, max /12)", Required: true},
	{Name: "server_certificate_arn", Type: core.ConnectionTypeString, Label: "Server Certificate ARN", Placeholder: "arn:aws:acm:...:certificate/...", Required: true},
	{Name: "authentication_type", Type: core.ConnectionTypeString, Label: "Authentication Type", Required: true, Options: []core.ConnectionOption{
		{Name: "Mutual (Certificate)", Value: "certificate-authentication"},
		{Name: "Active Directory", Value: "directory-service-authentication"},
		{Name: "Federated (SAML)", Value: "federated-authentication"},
	}},
	{Name: "root_certificate_chain_arn", Type: core.ConnectionTypeString, Label: "Client Root Certificate Chain ARN (for certificate auth)", Placeholder: "arn:aws:acm:...:certificate/..."},
	{Name: "connection_log_enabled", Type: core.ConnectionTypeBoolean, Label: "Enable Connection Logging", Required: true},
	{Name: "cloudwatch_log_group", Type: core.ConnectionTypeString, Label: "CloudWatch Log Group (if logging enabled)"},
	{Name: "vpc_id", Type: core.ConnectionTypeString, Label: "VPC ID (optional)", Placeholder: "vpc-0abc123"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description (optional)"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "endpoint", Type: core.ConnectionTypeObject, Label: "Client VPN Endpoint"},
	{Name: "client_vpn_endpoint_id", Type: core.ConnectionTypeString, Label: "Client VPN Endpoint ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	clientCIDR := strings.TrimSpace(awscommon.InputString("client_cidr_block", inputs))
	if clientCIDR == "" {
		return nil, fmt.Errorf("client_cidr_block is required")
	}
	serverCertArn := strings.TrimSpace(awscommon.InputString("server_certificate_arn", inputs))
	if serverCertArn == "" {
		return nil, fmt.Errorf("server_certificate_arn is required")
	}
	authType := strings.TrimSpace(awscommon.InputString("authentication_type", inputs))
	if authType == "" {
		return nil, fmt.Errorf("authentication_type is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	authReq := ec2types.ClientVpnAuthenticationRequest{Type: ec2types.ClientVpnAuthenticationType(authType)}
	switch authType {
	case string(ec2types.ClientVpnAuthenticationTypeCertificateAuthentication):
		if arn := strings.TrimSpace(awscommon.InputString("root_certificate_chain_arn", inputs)); arn != "" {
			authReq.MutualAuthentication = &ec2types.CertificateAuthenticationRequest{ClientRootCertificateChainArn: aws.String(arn)}
		}
	}

	logOpts := &ec2types.ConnectionLogOptions{Enabled: aws.Bool(awscommon.InputBool("connection_log_enabled", inputs))}
	if group := strings.TrimSpace(awscommon.InputString("cloudwatch_log_group", inputs)); group != "" {
		logOpts.CloudwatchLogGroup = aws.String(group)
	}

	in := &ec2.CreateClientVpnEndpointInput{
		ClientCidrBlock:       aws.String(clientCIDR),
		ServerCertificateArn:  aws.String(serverCertArn),
		AuthenticationOptions: []ec2types.ClientVpnAuthenticationRequest{authReq},
		ConnectionLogOptions:  logOpts,
	}
	if vpcID := strings.TrimSpace(awscommon.InputString("vpc_id", inputs)); vpcID != "" {
		in.VpcId = aws.String(vpcID)
	}
	if desc := strings.TrimSpace(awscommon.InputString("description", inputs)); desc != "" {
		in.Description = aws.String(desc)
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.TagSpecifications = []ec2types.TagSpecification{{
			ResourceType: ec2types.ResourceTypeClientVpnEndpoint,
			Tags:         tags,
		}}
	}

	out, err := client.CreateClientVpnEndpoint(ctx, in)
	if err != nil {
		return nil, err
	}

	id := aws.ToString(out.ClientVpnEndpointId)
	endpoint := map[string]interface{}{
		"client_vpn_endpoint_id": id,
		"dns_name":               aws.ToString(out.DnsName),
	}
	if out.Status != nil {
		endpoint["status"] = string(out.Status.Code)
		endpoint["status_message"] = aws.ToString(out.Status.Message)
	}

	return map[string]interface{}{
		"tool_result":            fmt.Sprintf("Created Client VPN endpoint %s", id),
		"endpoint":               endpoint,
		"client_vpn_endpoint_id": id,
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
