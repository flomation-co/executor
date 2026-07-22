// Package aws_secretsmanager_validate_resource_policy validates a resource-based policy for a secret.
package aws_secretsmanager_validate_resource_policy

import (
	"context"
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Secrets Manager Validate Resource Policy"
	Description  = "Validate a JSON resource-based policy for a secret before attaching it."
	Website      = "https://www.flomation.co"
	Icon         = "lock+circle-check"
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
	{Name: "resource_policy", Type: core.ConnectionTypeString, Label: "Resource Policy (JSON)", Placeholder: `{"Version":"2012-10-17","Statement":[...]}`, Required: true},
	{Name: "secret_id", Type: core.ConnectionTypeString, Label: "Secret ID or ARN (optional)", Placeholder: "my-secret"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "policy_validation_passed", Type: core.ConnectionTypeBoolean, Label: "Validation Passed"},
	{Name: "validation_errors", Type: core.ConnectionTypeString, Label: "Validation Errors (JSON)"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	policy := awscommon.InputString("resource_policy", inputs)
	if policy == "" {
		return nil, fmt.Errorf("resource policy is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := secretsmanager.NewFromConfig(cfg)

	in := &secretsmanager.ValidateResourcePolicyInput{ResourcePolicy: aws.String(policy)}
	if v := awscommon.InputString("secret_id", inputs); v != "" {
		in.SecretId = aws.String(v)
	}

	out, err := client.ValidateResourcePolicy(ctx, in)
	if err != nil {
		return nil, err
	}

	type validationError struct {
		CheckName    string `json:"check_name"`
		ErrorMessage string `json:"error_message"`
	}
	errsOut := make([]validationError, 0, len(out.ValidationErrors))
	for _, e := range out.ValidationErrors {
		errsOut = append(errsOut, validationError{
			CheckName:    aws.ToString(e.CheckName),
			ErrorMessage: aws.ToString(e.ErrorMessage),
		})
	}
	errsJSON, err := json.Marshal(errsOut)
	if err != nil {
		return nil, err
	}

	summary := "Resource policy validation passed"
	if !out.PolicyValidationPassed {
		summary = fmt.Sprintf("Resource policy validation failed with %d error(s)", len(errsOut))
	}

	return map[string]interface{}{
		"tool_result":              summary,
		"policy_validation_passed": out.PolicyValidationPassed,
		"validation_errors":        string(errsJSON),
	}, nil
}
