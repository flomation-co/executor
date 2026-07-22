// Package aws_secretsmanager_list_secret_version_ids lists the version IDs of a secret in AWS Secrets Manager.
package aws_secretsmanager_list_secret_version_ids

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
	Name         = "AWS Secrets Manager List Secret Version IDs"
	Description  = "List the version IDs and staging labels of a secret."
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
	{Name: "secret_id", Type: core.ConnectionTypeString, Label: "Secret ID or ARN", Placeholder: "prod/db/password", Required: true},
	{Name: "include_deprecated", Type: core.ConnectionTypeBoolean, Label: "Include Deprecated Versions"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "versions", Type: core.ConnectionTypeString, Label: "Versions (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

type versionEntry struct {
	VersionID     string   `json:"version_id"`
	VersionStages []string `json:"version_stages"`
	CreatedDate   string   `json:"created_date"`
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

	in := &secretsmanager.ListSecretVersionIdsInput{SecretId: aws.String(secretID)}
	if awscommon.InputBool("include_deprecated", inputs) {
		in.IncludeDeprecated = aws.Bool(true)
	}

	var versions []versionEntry
	paginator := secretsmanager.NewListSecretVersionIdsPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, v := range page.Versions {
			created := ""
			if v.CreatedDate != nil {
				created = v.CreatedDate.UTC().Format("2006-01-02T15:04:05Z")
			}
			versions = append(versions, versionEntry{
				VersionID:     aws.ToString(v.VersionId),
				VersionStages: v.VersionStages,
				CreatedDate:   created,
			})
		}
	}

	data, err := json.Marshal(versions)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d version(s) for %s", len(versions), secretID),
		"versions":    string(data),
		"count":       len(versions),
	}, nil
}
