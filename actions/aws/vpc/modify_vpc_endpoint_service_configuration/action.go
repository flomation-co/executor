// Package aws_vpc_modify_vpc_endpoint_service_configuration changes the settings
// of a VPC endpoint service you own.
package aws_vpc_modify_vpc_endpoint_service_configuration

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS VPC Modify Endpoint Service Configuration"
	Description  = "Change a VPC endpoint service: acceptance, load balancers or private DNS name."
	Website      = "https://www.flomation.co"
	Icon         = "link+pen"
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
	{Name: "service_id", Type: core.ConnectionTypeString, Label: "Service ID", Placeholder: "vpce-svc-0abc", Required: true},
	{Name: "acceptance_required", Type: core.ConnectionTypeBoolean, Label: "Require Acceptance (optional)"},
	{Name: "add_network_load_balancer_arns", Type: core.ConnectionTypeString, Label: "Add Network Load Balancer ARNs (optional)", Placeholder: "Comma-separated NLB ARNs"},
	{Name: "remove_network_load_balancer_arns", Type: core.ConnectionTypeString, Label: "Remove Network Load Balancer ARNs (optional)", Placeholder: "Comma-separated NLB ARNs"},
	{Name: "private_dns_name", Type: core.ConnectionTypeString, Label: "Private DNS Name (optional)", Placeholder: "service.example.com"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	serviceID := strings.TrimSpace(awscommon.InputString("service_id", inputs))
	if serviceID == "" {
		return nil, fmt.Errorf("service_id is required")
	}

	in := &ec2.ModifyVpcEndpointServiceConfigurationInput{
		ServiceId: aws.String(serviceID),
	}
	changed := false
	if c := core.FindConnection("acceptance_required", inputs); c != nil {
		if b := c.Boolean(); b != nil {
			in.AcceptanceRequired = aws.Bool(*b)
			changed = true
		}
	}
	if v := awscommon.InputStrings("add_network_load_balancer_arns", inputs); len(v) > 0 {
		in.AddNetworkLoadBalancerArns = v
		changed = true
	}
	if v := awscommon.InputStrings("remove_network_load_balancer_arns", inputs); len(v) > 0 {
		in.RemoveNetworkLoadBalancerArns = v
		changed = true
	}
	if v := strings.TrimSpace(awscommon.InputString("private_dns_name", inputs)); v != "" {
		in.PrivateDnsName = aws.String(v)
		changed = true
	}
	if !changed {
		return nil, fmt.Errorf("at least one change is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	out, err := client.ModifyVpcEndpointServiceConfiguration(ctx, in)
	if err != nil {
		return nil, err
	}

	ok := aws.ToBool(out.Return)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Modified VPC endpoint service configuration %s (success=%t)", serviceID, ok),
		"success":     ok,
	}, nil
}
