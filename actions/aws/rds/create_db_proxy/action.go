// Package aws_rds_create_db_proxy creates an RDS Proxy fronting a database,
// using a Secrets Manager secret for the database credentials.
package aws_rds_create_db_proxy

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Create DB Proxy"
	Description  = "Create an RDS Proxy fronting a database with Secrets Manager credentials."
	Website      = "https://www.flomation.co"
	Icon         = "route+plus"
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
	{Name: "engine_family", Type: core.ConnectionTypeString, Label: "Engine Family", Required: true, Options: []core.ConnectionOption{
		{Name: "MySQL / MariaDB", Value: "MYSQL"},
		{Name: "PostgreSQL", Value: "POSTGRESQL"},
		{Name: "SQL Server", Value: "SQLSERVER"},
	}},
	{Name: "role_arn", Type: core.ConnectionTypeString, Label: "IAM Role ARN", Placeholder: "IAM role granting Secrets Manager access", Required: true},
	{Name: "vpc_subnet_ids", Type: core.ConnectionTypeString, Label: "VPC Subnet IDs", Placeholder: "Comma-separated, e.g. subnet-abc123,subnet-def456", Required: true},
	{Name: "secret_arn", Type: core.ConnectionTypeString, Label: "Secret ARN", Placeholder: "Secrets Manager secret with the DB credentials", Required: true},
	{Name: "require_tls", Type: core.ConnectionTypeBoolean, Label: "Require TLS"},
	{Name: "idle_client_timeout", Type: core.ConnectionTypeInteger, Label: "Idle Client Timeout (seconds, optional)"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags (optional)"},
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
	family := awscommon.InputString("engine_family", inputs)
	if family == "" {
		return nil, fmt.Errorf("engine family is required")
	}
	roleArn := awscommon.InputString("role_arn", inputs)
	if roleArn == "" {
		return nil, fmt.Errorf("IAM role ARN is required")
	}
	subnets := awscommon.InputStrings("vpc_subnet_ids", inputs)
	if len(subnets) == 0 {
		return nil, fmt.Errorf("at least one VPC subnet ID is required")
	}
	secretArn := awscommon.InputString("secret_arn", inputs)
	if secretArn == "" {
		return nil, fmt.Errorf("secret ARN is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.CreateDBProxyInput{
		DBProxyName:  aws.String(name),
		EngineFamily: rdstypes.EngineFamily(family),
		RoleArn:      aws.String(roleArn),
		VpcSubnetIds: subnets,
		Auth: []rdstypes.UserAuthConfig{{
			AuthScheme: rdstypes.AuthSchemeSecrets,
			SecretArn:  aws.String(secretArn),
			IAMAuth:    rdstypes.IAMAuthModeDisabled,
		}},
	}
	if awscommon.InputBool("require_tls", inputs) {
		in.RequireTLS = aws.Bool(true)
	}
	if n, ok := awscommon.InputInt("idle_client_timeout", inputs); ok {
		in.IdleClientTimeout = aws.Int32(int32(n))
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.Tags = tags
	}

	out, err := client.CreateDBProxy(ctx, in)
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
		"tool_result": fmt.Sprintf("Created DB proxy %q (%s)", name, string(out.DBProxy.Status)),
		"proxy":       proxy,
	}, nil
}

func buildTags(inputs []*core.Connection) []rdstypes.Tag {
	conn := core.FindConnection("tags", inputs)
	if conn == nil {
		return nil
	}
	var tags []rdstypes.Tag
	for _, kv := range conn.KeyValuePairs() {
		k := strings.TrimSpace(kv.Key)
		if k == "" {
			continue
		}
		tags = append(tags, rdstypes.Tag{Key: aws.String(k), Value: aws.String(kv.Value)})
	}
	return tags
}
