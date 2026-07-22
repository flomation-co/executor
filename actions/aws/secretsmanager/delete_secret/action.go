// Package aws_secretsmanager_delete_secret deletes a secret from AWS Secrets Manager.
package aws_secretsmanager_delete_secret

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Secrets Manager Delete Secret"
	Description  = "Schedule a secret for deletion, with an optional recovery window."
	Website      = "https://www.flomation.co"
	Icon         = "lock+trash"
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
	{Name: "secret_id", Type: core.ConnectionTypeString, Label: "Secret ID or ARN", Placeholder: "prod/db/password", Required: true},
	{Name: "recovery_window_in_days", Type: core.ConnectionTypeInteger, Label: "Recovery Window (days, optional)", Placeholder: "30"},
	{Name: "force_delete_without_recovery", Type: core.ConnectionTypeBoolean, Label: "Force Delete Without Recovery"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "arn", Type: core.ConnectionTypeString, Label: "Secret ARN"},
	{Name: "deletion_date", Type: core.ConnectionTypeString, Label: "Deletion Date"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	secretID := awscommon.InputString("secret_id", inputs)
	if secretID == "" {
		return nil, fmt.Errorf("secret id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := secretsmanager.NewFromConfig(cfg)

	in := &secretsmanager.DeleteSecretInput{SecretId: aws.String(secretID)}
	if n, ok := awscommon.InputInt("recovery_window_in_days", inputs); ok {
		in.RecoveryWindowInDays = aws.Int64(n)
	}
	if awscommon.InputBool("force_delete_without_recovery", inputs) {
		in.ForceDeleteWithoutRecovery = aws.Bool(true)
	}

	out, err := client.DeleteSecret(ctx, in)
	if err != nil {
		return nil, err
	}

	deletionDate := ""
	if out.DeletionDate != nil {
		deletionDate = out.DeletionDate.UTC().Format("2006-01-02T15:04:05Z")
	}

	arn := aws.ToString(out.ARN)
	return map[string]interface{}{
		"tool_result":   fmt.Sprintf("Scheduled deletion of %s", aws.ToString(out.Name)),
		"arn":           arn,
		"deletion_date": deletionDate,
	}, nil
}
