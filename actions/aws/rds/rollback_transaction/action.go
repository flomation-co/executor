// Package aws_rds_rollback_transaction rolls back an Aurora Data API transaction.
package aws_rds_rollback_transaction

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rdsdata"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Rollback Transaction"
	Description  = "Roll back an Aurora Data API transaction started with Begin Transaction."
	Website      = "https://www.flomation.co"
	Icon         = "database+rotate"
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
	{Name: "transaction_id", Type: core.ConnectionTypeString, Label: "Transaction ID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Transaction Status"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	resourceArn := awscommon.InputString("resource_arn", inputs)
	secretArn := awscommon.InputString("secret_arn", inputs)
	txID := awscommon.InputString("transaction_id", inputs)
	if resourceArn == "" || secretArn == "" || txID == "" {
		return nil, fmt.Errorf("cluster ARN, secret ARN and transaction ID are required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rdsdata.NewFromConfig(cfg)

	out, err := client.RollbackTransaction(ctx, &rdsdata.RollbackTransactionInput{
		ResourceArn:   aws.String(resourceArn),
		SecretArn:     aws.String(secretArn),
		TransactionId: aws.String(txID),
	})
	if err != nil {
		return nil, err
	}

	status := aws.ToString(out.TransactionStatus)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Transaction rolled back (%s)", status),
		"status":      status,
	}, nil
}
