// Package aws_route53_update_health_check updates a Route 53 health check.
package aws_route53_update_health_check

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
	Name         = "AWS Route 53 Update Health Check"
	Description  = "Update the settings of an existing Route 53 health check."
	Website      = "https://www.flomation.co"
	Icon         = "gauge+pen"
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
	{Name: "ip_address", Type: core.ConnectionTypeString, Label: "IP Address", Placeholder: "Optional — 192.0.2.44"},
	{Name: "fully_qualified_domain_name", Type: core.ConnectionTypeString, Label: "Fully Qualified Domain Name", Placeholder: "Optional — www.example.com"},
	{Name: "port", Type: core.ConnectionTypeInteger, Label: "Port", Placeholder: "Optional — 443"},
	{Name: "resource_path", Type: core.ConnectionTypeString, Label: "Resource Path", Placeholder: "Optional — /health"},
	{Name: "failure_threshold", Type: core.ConnectionTypeInteger, Label: "Failure Threshold", Placeholder: "Optional — e.g. 3"},
	{Name: "search_string", Type: core.ConnectionTypeString, Label: "Search String", Placeholder: "Optional — for *_STR_MATCH types"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "health_check_id", Type: core.ConnectionTypeString, Label: "Health Check ID"},
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

	in := &route53.UpdateHealthCheckInput{HealthCheckId: aws.String(id)}
	if v := awscommon.InputString("ip_address", inputs); v != "" {
		in.IPAddress = aws.String(v)
	}
	if v := awscommon.InputString("fully_qualified_domain_name", inputs); v != "" {
		in.FullyQualifiedDomainName = aws.String(v)
	}
	if v := awscommon.InputString("resource_path", inputs); v != "" {
		in.ResourcePath = aws.String(v)
	}
	if v := awscommon.InputString("search_string", inputs); v != "" {
		in.SearchString = aws.String(v)
	}
	if v, ok := awscommon.InputInt("port", inputs); ok {
		in.Port = aws.Int32(int32(v))
	}
	if v, ok := awscommon.InputInt("failure_threshold", inputs); ok {
		in.FailureThreshold = aws.Int32(int32(v))
	}

	_, err = client.UpdateHealthCheck(ctx, in)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Updated health check %s", id),
		"health_check_id": id,
	}, nil
}
