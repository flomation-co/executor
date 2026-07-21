// Package aws_s3_put_storage_lens_configuration creates or updates an S3 Storage Lens configuration.
package aws_s3_put_storage_lens_configuration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3control"
	s3ctltypes "github.com/aws/aws-sdk-go-v2/service/s3control/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS S3 Put Storage Lens Configuration"
	Description  = "Create or update an S3 Storage Lens configuration (JSON uses SDK field names)."
	Website      = "https://www.flomation.co"
	Icon         = "chart-line+pen"
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
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "AWS Account ID", Placeholder: "12-digit account ID; leave blank to auto-detect from the credential"},
	{Name: "config_id", Type: core.ConnectionTypeString, Label: "Configuration ID", Required: true},
	{Name: "storage_lens_configuration", Type: core.ConnectionTypeString, Label: "Storage Lens Configuration (JSON)", Placeholder: "JSON object; keys use SDK field names (Id, IsEnabled, AccountLevel, ...)", Required: true},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "config_id", Type: core.ConnectionTypeString, Label: "Configuration ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	configID := strings.TrimSpace(awscommon.InputString("config_id", inputs))
	if configID == "" {
		return nil, fmt.Errorf("config_id is required")
	}
	raw := strings.TrimSpace(awscommon.InputString("storage_lens_configuration", inputs))
	if raw == "" {
		return nil, fmt.Errorf("storage_lens_configuration JSON is required")
	}
	var slc s3ctltypes.StorageLensConfiguration
	if err := json.Unmarshal([]byte(raw), &slc); err != nil {
		return nil, fmt.Errorf("invalid storage_lens_configuration JSON: %w", err)
	}
	if slc.Id == nil || aws.ToString(slc.Id) == "" {
		slc.Id = aws.String(configID)
	}

	var tags []s3ctltypes.StorageLensTag
	if conn := core.FindConnection("tags", inputs); conn != nil {
		for _, kv := range conn.KeyValuePairs() {
			k := strings.TrimSpace(kv.Key)
			if k == "" {
				continue
			}
			tags = append(tags, s3ctltypes.StorageLensTag{Key: aws.String(k), Value: aws.String(strings.TrimSpace(kv.Value))})
		}
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	accountID, err := awscommon.ResolveAccountID(ctx, cfg, inputs)
	if err != nil {
		return nil, err
	}
	client := s3control.NewFromConfig(cfg)

	_, err = client.PutStorageLensConfiguration(ctx, &s3control.PutStorageLensConfigurationInput{
		AccountId:                aws.String(accountID),
		ConfigId:                 aws.String(configID),
		StorageLensConfiguration: &slc,
		Tags:                     tags,
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Saved Storage Lens configuration %s", configID),
		"config_id":   configID,
	}, nil
}
