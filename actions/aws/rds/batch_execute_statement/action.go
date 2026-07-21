// Package aws_rds_batch_execute_statement runs one SQL statement against Aurora
// Serverless with many parameter sets via the RDS Data API.
package aws_rds_batch_execute_statement

import (
	"context"
	"encoding/json"
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
	Name         = "AWS RDS Batch Execute Statement"
	Description  = "Run one SQL statement with many parameter sets against Aurora Serverless via the Data API."
	Website      = "https://www.flomation.co"
	Icon         = "database+list"
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
	{Name: "secret_arn", Type: core.ConnectionTypeString, Label: "Secrets Manager Secret ARN", Required: true},
	{Name: "database", Type: core.ConnectionTypeString, Label: "Database Name (optional)"},
	{Name: "sql", Type: core.ConnectionTypeText, Label: "SQL", Placeholder: "INSERT INTO orders (ref, status) VALUES (:ref, :status)", Required: true},
	{Name: "parameter_sets", Type: core.ConnectionTypeText, Label: "Parameter Sets (JSON)", Placeholder: "[{\"ref\":\"A1\",\"status\":\"paid\"},{\"ref\":\"A2\",\"status\":\"pending\"}]", Required: true},
	{Name: "transaction_id", Type: core.ConnectionTypeString, Label: "Transaction ID (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "sets_applied", Type: core.ConnectionTypeInteger, Label: "Parameter Sets Applied"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	resourceArn := awscommon.InputString("resource_arn", inputs)
	secretArn := awscommon.InputString("secret_arn", inputs)
	sql := awscommon.InputString("sql", inputs)
	rawSets := strings.TrimSpace(awscommon.InputString("parameter_sets", inputs))
	if resourceArn == "" || secretArn == "" || sql == "" || rawSets == "" {
		return nil, fmt.Errorf("cluster ARN, secret ARN, SQL and parameter sets are all required")
	}

	var rows []map[string]interface{}
	if err := json.Unmarshal([]byte(rawSets), &rows); err != nil {
		return nil, fmt.Errorf("parameter_sets must be a JSON array of objects: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("parameter_sets is empty")
	}

	paramSets := make([][]rdsdatatypes.SqlParameter, 0, len(rows))
	for _, row := range rows {
		var set []rdsdatatypes.SqlParameter
		for name, val := range row {
			set = append(set, rdsdatatypes.SqlParameter{
				Name:  aws.String(name),
				Value: &rdsdatatypes.FieldMemberStringValue{Value: fmt.Sprintf("%v", val)},
			})
		}
		paramSets = append(paramSets, set)
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rdsdata.NewFromConfig(cfg)

	in := &rdsdata.BatchExecuteStatementInput{
		ResourceArn:   aws.String(resourceArn),
		SecretArn:     aws.String(secretArn),
		Sql:           aws.String(sql),
		ParameterSets: paramSets,
	}
	if v := awscommon.InputString("database", inputs); v != "" {
		in.Database = aws.String(v)
	}
	if v := awscommon.InputString("transaction_id", inputs); v != "" {
		in.TransactionId = aws.String(v)
	}

	if _, err := client.BatchExecuteStatement(ctx, in); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Batch executed with %d parameter set(s)", len(paramSets)),
		"sets_applied": len(paramSets),
	}, nil
}
