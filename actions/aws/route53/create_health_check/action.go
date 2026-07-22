// Package aws_route53_create_health_check creates a Route 53 health check.
package aws_route53_create_health_check

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Route 53 Create Health Check"
	Description  = "Create a Route 53 health check monitoring an endpoint, domain or alarm."
	Website      = "https://www.flomation.co"
	Icon         = "gauge+plus"
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
	{Name: "caller_reference", Type: core.ConnectionTypeString, Label: "Caller Reference", Placeholder: "Optional — unique idempotency token (auto-generated if blank)"},
	{Name: "type", Type: core.ConnectionTypeString, Label: "Type", Required: true, Options: []core.ConnectionOption{
		{Name: "HTTP", Value: "HTTP"},
		{Name: "HTTPS", Value: "HTTPS"},
		{Name: "HTTP String Match", Value: "HTTP_STR_MATCH"},
		{Name: "HTTPS String Match", Value: "HTTPS_STR_MATCH"},
		{Name: "TCP", Value: "TCP"},
		{Name: "Calculated", Value: "CALCULATED"},
		{Name: "CloudWatch Metric", Value: "CLOUDWATCH_METRIC"},
	}},
	{Name: "ip_address", Type: core.ConnectionTypeString, Label: "IP Address", Placeholder: "Optional — 192.0.2.44"},
	{Name: "fully_qualified_domain_name", Type: core.ConnectionTypeString, Label: "Fully Qualified Domain Name", Placeholder: "Optional — www.example.com"},
	{Name: "port", Type: core.ConnectionTypeInteger, Label: "Port", Placeholder: "Optional — 443"},
	{Name: "resource_path", Type: core.ConnectionTypeString, Label: "Resource Path", Placeholder: "Optional — /health"},
	{Name: "request_interval", Type: core.ConnectionTypeInteger, Label: "Request Interval (seconds)", Placeholder: "Optional — 10 or 30"},
	{Name: "failure_threshold", Type: core.ConnectionTypeInteger, Label: "Failure Threshold", Placeholder: "Optional — e.g. 3"},
	{Name: "search_string", Type: core.ConnectionTypeString, Label: "Search String", Placeholder: "Optional — required for *_STR_MATCH types"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "health_check_id", Type: core.ConnectionTypeString, Label: "Health Check ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	hcType := awscommon.InputString("type", inputs)
	if hcType == "" {
		return nil, fmt.Errorf("type is required")
	}

	callerRef := awscommon.InputString("caller_reference", inputs)
	fqdn := awscommon.InputString("fully_qualified_domain_name", inputs)
	ip := awscommon.InputString("ip_address", inputs)
	resourcePath := awscommon.InputString("resource_path", inputs)
	searchString := awscommon.InputString("search_string", inputs)

	if callerRef == "" {
		// CallerReference must be non-empty and unique. Derive a stable value
		// from the endpoint identity so retries of the same request collapse.
		callerRef = fmt.Sprintf("flomation-%s-%s-%s", hcType, fqdn, ip)
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := route53.NewFromConfig(cfg)

	hcConfig := &r53types.HealthCheckConfig{Type: r53types.HealthCheckType(hcType)}
	if ip != "" {
		hcConfig.IPAddress = aws.String(ip)
	}
	if fqdn != "" {
		hcConfig.FullyQualifiedDomainName = aws.String(fqdn)
	}
	if resourcePath != "" {
		hcConfig.ResourcePath = aws.String(resourcePath)
	}
	if searchString != "" {
		hcConfig.SearchString = aws.String(searchString)
	}
	if v, ok := awscommon.InputInt("port", inputs); ok {
		hcConfig.Port = aws.Int32(int32(v))
	}
	if v, ok := awscommon.InputInt("request_interval", inputs); ok {
		hcConfig.RequestInterval = aws.Int32(int32(v))
	}
	if v, ok := awscommon.InputInt("failure_threshold", inputs); ok {
		hcConfig.FailureThreshold = aws.Int32(int32(v))
	}

	out, err := client.CreateHealthCheck(ctx, &route53.CreateHealthCheckInput{
		CallerReference:   aws.String(callerRef),
		HealthCheckConfig: hcConfig,
	})
	if err != nil {
		return nil, err
	}

	var id string
	if out.HealthCheck != nil {
		id = aws.ToString(out.HealthCheck.Id)
	}
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Created %s health check %s", hcType, id),
		"health_check_id": id,
	}, nil
}
