// Package aws_route53_get_health_check retrieves a Route 53 health check.
package aws_route53_get_health_check

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Route 53 Get Health Check"
	Description  = "Fetch the configuration of a Route 53 health check by ID."
	Website      = "https://www.flomation.co"
	Icon         = "gauge+magnifying-glass"
	Date         = "22/07/2026"
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
	{Name: "health_check_id", Type: core.ConnectionTypeString, Label: "Health Check ID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "type", Type: core.ConnectionTypeString, Label: "Type"},
	{Name: "fully_qualified_domain_name", Type: core.ConnectionTypeString, Label: "Fully Qualified Domain Name"},
	{Name: "ip_address", Type: core.ConnectionTypeString, Label: "IP Address"},
	{Name: "port", Type: core.ConnectionTypeInteger, Label: "Port"},
	{Name: "resource_path", Type: core.ConnectionTypeString, Label: "Resource Path"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	id := awscommon.InputString("health_check_id", inputs)
	if id == "" {
		return nil, fmt.Errorf("health check id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := route53.NewFromConfig(cfg)

	out, err := client.GetHealthCheck(ctx, &route53.GetHealthCheckInput{HealthCheckId: aws.String(id)})
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"tool_result":                 fmt.Sprintf("Retrieved health check %s", id),
		"type":                        "",
		"fully_qualified_domain_name": "",
		"ip_address":                  "",
		"port":                        nil,
		"resource_path":               "",
	}
	if out.HealthCheck != nil && out.HealthCheck.HealthCheckConfig != nil {
		c := out.HealthCheck.HealthCheckConfig
		result["type"] = string(c.Type)
		result["fully_qualified_domain_name"] = aws.ToString(c.FullyQualifiedDomainName)
		result["ip_address"] = aws.ToString(c.IPAddress)
		if c.Port != nil {
			result["port"] = int(*c.Port)
		}
		result["resource_path"] = aws.ToString(c.ResourcePath)
	}
	return result, nil
}
