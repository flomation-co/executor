// Package aws_secretsmanager_describe_secret retrieves the metadata of a secret in AWS Secrets Manager.
package aws_secretsmanager_describe_secret

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
	Name         = "AWS Secrets Manager Describe Secret"
	Description  = "Retrieve the metadata of a secret without exposing its value."
	Website      = "https://www.flomation.co"
	Icon         = "lock+magnifying-glass"
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "arn", Type: core.ConnectionTypeString, Label: "Secret ARN"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Secret Name"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description"},
	{Name: "rotation_enabled", Type: core.ConnectionTypeBoolean, Label: "Rotation Enabled"},
	{Name: "last_changed_date", Type: core.ConnectionTypeString, Label: "Last Changed Date"},
	{Name: "kms_key_id", Type: core.ConnectionTypeString, Label: "KMS Key ID"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Tags (JSON)"},
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

	out, err := client.DescribeSecret(ctx, &secretsmanager.DescribeSecretInput{SecretId: aws.String(secretID)})
	if err != nil {
		return nil, err
	}

	type tagEntry struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	tags := make([]tagEntry, 0, len(out.Tags))
	for _, t := range out.Tags {
		tags = append(tags, tagEntry{Key: aws.ToString(t.Key), Value: aws.ToString(t.Value)})
	}
	tagsData, err := json.Marshal(tags)
	if err != nil {
		return nil, err
	}

	lastChanged := ""
	if out.LastChangedDate != nil {
		lastChanged = out.LastChangedDate.UTC().Format("2006-01-02T15:04:05Z")
	}

	arn := aws.ToString(out.ARN)
	return map[string]interface{}{
		"tool_result":       fmt.Sprintf("Secret %s (%s)", aws.ToString(out.Name), arn),
		"arn":               arn,
		"name":              aws.ToString(out.Name),
		"description":       aws.ToString(out.Description),
		"rotation_enabled":  aws.ToBool(out.RotationEnabled),
		"last_changed_date": lastChanged,
		"kms_key_id":        aws.ToString(out.KmsKeyId),
		"tags":              string(tagsData),
	}, nil
}
