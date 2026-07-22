// Package aws_kms_list_aliases lists KMS aliases.
package aws_kms_list_aliases

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS KMS List Aliases"
	Description  = "List KMS aliases, optionally filtered to a single key."
	Website      = "https://www.flomation.co"
	Icon         = "key+list"
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
	{Name: "key_id", Type: core.ConnectionTypeString, Label: "Key ID / ARN (optional)", Placeholder: "Filter aliases to a single key"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "aliases", Type: core.ConnectionTypeString, Label: "Aliases (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

type aliasEntry struct {
	AliasName   string `json:"alias_name"`
	TargetKeyID string `json:"target_key_id"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := kms.NewFromConfig(cfg)

	in := &kms.ListAliasesInput{}
	if k := strings.TrimSpace(awscommon.InputString("key_id", inputs)); k != "" {
		in.KeyId = aws.String(k)
	}

	var aliases []aliasEntry
	paginator := kms.NewListAliasesPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, a := range page.Aliases {
			aliases = append(aliases, aliasEntry{
				AliasName:   aws.ToString(a.AliasName),
				TargetKeyID: aws.ToString(a.TargetKeyId),
			})
		}
	}

	data, err := json.Marshal(aliases)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d alias(es)", len(aliases)),
		"aliases":     string(data),
		"count":       len(aliases),
	}, nil
}
