// Package aws_rds_delete_db_proxy deletes an existing RDS Proxy.
package aws_rds_delete_db_proxy

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Delete DB Proxy"
	Description  = "Delete an existing RDS Proxy."
	Website      = "https://www.flomation.co"
	Icon         = "route+trash"
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

	out, err := client.DeleteDBProxy(ctx, &rds.DeleteDBProxyInput{DBProxyName: aws.String(name)})
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
		"tool_result": fmt.Sprintf("Deleted DB proxy %q", name),
		"proxy":       proxy,
	}, nil
}
