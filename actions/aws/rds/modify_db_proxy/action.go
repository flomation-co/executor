// Package aws_rds_modify_db_proxy updates the configuration of an existing RDS
// Proxy (name, TLS requirement, idle timeout, debug logging).
package aws_rds_modify_db_proxy

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Modify DB Proxy"
	Description  = "Update an existing RDS Proxy's name, TLS, idle timeout or debug logging."
	Website      = "https://www.flomation.co"
	Icon         = "route+pen"
	Date         = "20/07/2026"
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
	{Name: "db_proxy_name", Type: core.ConnectionTypeString, Label: "DB Proxy Name", Placeholder: "my-proxy", Required: true},
	{Name: "new_db_proxy_name", Type: core.ConnectionTypeString, Label: "New DB Proxy Name (optional)"},
	{Name: "require_tls", Type: core.ConnectionTypeString, Label: "Require TLS (optional)", Options: []core.ConnectionOption{
		{Name: "Leave unchanged", Value: ""},
		{Name: "Enable", Value: "true"},
		{Name: "Disable", Value: "false"},
	}},
	{Name: "idle_client_timeout", Type: core.ConnectionTypeInteger, Label: "Idle Client Timeout (seconds, optional)"},
	{Name: "debug_logging", Type: core.ConnectionTypeString, Label: "Debug Logging (optional)", Options: []core.ConnectionOption{
		{Name: "Leave unchanged", Value: ""},
		{Name: "Enable", Value: "true"},
		{Name: "Disable", Value: "false"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "proxy", Type: core.ConnectionTypeObject, Label: "DB Proxy"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	name := awscommon.InputString("db_proxy_name", inputs)
	if name == "" {
		return nil, fmt.Errorf("db proxy name is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.ModifyDBProxyInput{DBProxyName: aws.String(name)}
	if v := awscommon.InputString("new_db_proxy_name", inputs); v != "" {
		in.NewDBProxyName = aws.String(v)
	}
	if v := strings.ToLower(strings.TrimSpace(awscommon.InputString("require_tls", inputs))); v != "" {
		in.RequireTLS = aws.Bool(v == "true")
	}
	if n, ok := awscommon.InputInt("idle_client_timeout", inputs); ok {
		in.IdleClientTimeout = aws.Int32(int32(n))
	}
	if v := strings.ToLower(strings.TrimSpace(awscommon.InputString("debug_logging", inputs))); v != "" {
		in.DebugLogging = aws.Bool(v == "true")
	}

	out, err := client.ModifyDBProxy(ctx, in)
	if err != nil {
		return nil, err
	}

	var proxy map[string]interface{}
	if p := out.DBProxy; p != nil {
		proxy = map[string]interface{}{
			"db_proxy_name": aws.ToString(p.DBProxyName),
			"status":        string(p.Status),
			"endpoint":      aws.ToString(p.Endpoint),
			"engine_family": aws.ToString(p.EngineFamily),
			"arn":           aws.ToString(p.DBProxyArn),
		}
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Modified DB proxy %q", name),
		"proxy":       proxy,
	}, nil
}
