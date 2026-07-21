// Package aws_rds_execute_statement runs a single SQL statement against an Aurora
// Serverless database via the RDS Data API (HTTP, no persistent connection).
package aws_rds_execute_statement

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rdsdata"
	rdsdatatypes "github.com/aws/aws-sdk-go-v2/service/rdsdata/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Execute Statement"
	Description  = "Run a SQL statement against Aurora Serverless via the Data API (no persistent connection)."
	Website      = "https://www.flomation.co"
	Icon         = "database+terminal"
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
	{Name: "resource_arn", Type: core.ConnectionTypeString, Label: "Aurora Cluster ARN", Placeholder: "arn:aws:rds:eu-west-2:123456789012:cluster:my-aurora", Required: true},
	{Name: "secret_arn", Type: core.ConnectionTypeString, Label: "Secrets Manager Secret ARN", Placeholder: "arn:aws:secretsmanager:...:secret:my-db-creds", Required: true},
	{Name: "database", Type: core.ConnectionTypeString, Label: "Database Name (optional)"},
	{Name: "sql", Type: core.ConnectionTypeText, Label: "SQL", Placeholder: "SELECT * FROM orders WHERE status = :status", Required: true},
	{Name: "parameters", Type: core.ConnectionTypeKeyValueArray, Label: "Named Parameters (optional)", Placeholder: "status = paid — bound safely as :status"},
	{Name: "transaction_id", Type: core.ConnectionTypeString, Label: "Transaction ID (optional)", Placeholder: "From Begin Transaction, to run inside a transaction"},
	{Name: "continue_after_timeout", Type: core.ConnectionTypeBoolean, Label: "Continue After Timeout (for long DML)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "records", Type: core.ConnectionTypeString, Label: "Records (JSON)"},
	{Name: "rows_updated", Type: core.ConnectionTypeInteger, Label: "Rows Updated"},
	{Name: "generated_fields", Type: core.ConnectionTypeObject, Label: "Generated Fields"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	resourceArn := awscommon.InputString("resource_arn", inputs)
	secretArn := awscommon.InputString("secret_arn", inputs)
	sql := awscommon.InputString("sql", inputs)
	if resourceArn == "" || secretArn == "" || sql == "" {
		return nil, fmt.Errorf("cluster ARN, secret ARN and SQL are all required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rdsdata.NewFromConfig(cfg)

	in := &rdsdata.ExecuteStatementInput{
		ResourceArn:          aws.String(resourceArn),
		SecretArn:            aws.String(secretArn),
		Sql:                  aws.String(sql),
		FormatRecordsAs:      rdsdatatypes.RecordsFormatTypeJson,
		ContinueAfterTimeout: awscommon.InputBool("continue_after_timeout", inputs),
	}
	if v := awscommon.InputString("database", inputs); v != "" {
		in.Database = aws.String(v)
	}
	if v := awscommon.InputString("transaction_id", inputs); v != "" {
		in.TransactionId = aws.String(v)
	}
	if params := stringParameters(inputs); len(params) > 0 {
		in.Parameters = params
	}

	out, err := client.ExecuteStatement(ctx, in)
	if err != nil {
		return nil, err
	}

	records := aws.ToString(out.FormattedRecords)
	if records == "" {
		records = "[]"
	}
	var generated []interface{}
	for _, f := range out.GeneratedFields {
		generated = append(generated, fieldValue(f))
	}

	summary := fmt.Sprintf("Statement executed (%d row(s) updated)", out.NumberOfRecordsUpdated)
	if strings.TrimSpace(records) != "[]" {
		summary = "Statement executed; records returned as JSON"
	}

	return map[string]interface{}{
		"tool_result":      summary,
		"records":          records,
		"rows_updated":     out.NumberOfRecordsUpdated,
		"generated_fields": generated,
	}, nil
}

// stringParameters binds the named-parameters key_value_array as SQL string
// parameters — bound values, so `:name` placeholders defeat injection.
func stringParameters(inputs []*core.Connection) []rdsdatatypes.SqlParameter {
	conn := core.FindConnection("parameters", inputs)
	if conn == nil {
		return nil
	}
	var params []rdsdatatypes.SqlParameter
	for _, kv := range conn.KeyValuePairs() {
		name := strings.TrimSpace(kv.Key)
		if name == "" {
			continue
		}
		params = append(params, rdsdatatypes.SqlParameter{
			Name:  aws.String(name),
			Value: &rdsdatatypes.FieldMemberStringValue{Value: kv.Value},
		})
	}
	return params
}

// fieldValue unwraps a Data API Field union into a plain Go value.
func fieldValue(f rdsdatatypes.Field) interface{} {
	switch v := f.(type) {
	case *rdsdatatypes.FieldMemberStringValue:
		return v.Value
	case *rdsdatatypes.FieldMemberLongValue:
		return v.Value
	case *rdsdatatypes.FieldMemberDoubleValue:
		return v.Value
	case *rdsdatatypes.FieldMemberBooleanValue:
		return v.Value
	case *rdsdatatypes.FieldMemberIsNull:
		return nil
	default:
		return nil
	}
}
