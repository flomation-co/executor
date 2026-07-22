// Package aws_secretsmanager_batch_get_secret_value retrieves the values of multiple secrets in one call.
package aws_secretsmanager_batch_get_secret_value

import (
	"context"
	"encoding/base64"
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
	Name         = "AWS Secrets Manager Batch Get Secret Value"
	Description  = "Retrieve the values of multiple secrets at once. Returns sensitive data."
	Website      = "https://www.flomation.co"
	Icon         = "lock+list"
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
	{Name: "secret_id_list", Type: core.ConnectionTypeString, Label: "Secret IDs (comma-separated or JSON array)", Placeholder: "prod/db, prod/api", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "secret_values", Type: core.ConnectionTypeString, Label: "Secret Values (JSON, sensitive)"},
	{Name: "errors", Type: core.ConnectionTypeString, Label: "Errors (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	ids := awscommon.InputStrings("secret_id_list", inputs)
	if len(ids) == 0 {
		return nil, fmt.Errorf("secret id list is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := secretsmanager.NewFromConfig(cfg)

	in := &secretsmanager.BatchGetSecretValueInput{SecretIdList: ids}

	out, err := client.BatchGetSecretValue(ctx, in)
	if err != nil {
		return nil, err
	}

	type secretValue struct {
		Name         string `json:"name"`
		ARN          string `json:"arn"`
		SecretString string `json:"secret_string"`
		SecretBinary string `json:"secret_binary,omitempty"`
	}
	values := make([]secretValue, 0, len(out.SecretValues))
	for _, v := range out.SecretValues {
		sv := secretValue{
			Name:         aws.ToString(v.Name),
			ARN:          aws.ToString(v.ARN),
			SecretString: aws.ToString(v.SecretString),
		}
		if len(v.SecretBinary) > 0 {
			sv.SecretBinary = base64.StdEncoding.EncodeToString(v.SecretBinary)
		}
		values = append(values, sv)
	}
	valuesJSON, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}

	type apiError struct {
		SecretID  string `json:"secret_id"`
		ErrorCode string `json:"error_code"`
		Message   string `json:"message"`
	}
	errsOut := make([]apiError, 0, len(out.Errors))
	for _, e := range out.Errors {
		errsOut = append(errsOut, apiError{
			SecretID:  aws.ToString(e.SecretId),
			ErrorCode: aws.ToString(e.ErrorCode),
			Message:   aws.ToString(e.Message),
		})
	}
	errsJSON, err := json.Marshal(errsOut)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result":   fmt.Sprintf("Retrieved %d secret value(s), %d error(s)", len(values), len(errsOut)),
		"secret_values": string(valuesJSON),
		"errors":        string(errsJSON),
		"count":         len(values),
	}, nil
}
