// Package aws_kms_list_grants lists the grants on a KMS key.
package aws_kms_list_grants

import (
	"context"
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS KMS List Grants"
	Description  = "List the grants attached to a KMS key."
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
	{Name: "key_id", Type: core.ConnectionTypeString, Label: "Key ID or ARN", Placeholder: "1234abcd-12ab-34cd-56ef-1234567890ab", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "grants", Type: core.ConnectionTypeString, Label: "Grants (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	keyID := awscommon.InputString("key_id", inputs)
	if keyID == "" {
		return nil, fmt.Errorf("key_id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := kms.NewFromConfig(cfg)

	type grantView struct {
		GrantID          string   `json:"grant_id"`
		GranteePrincipal string   `json:"grantee_principal"`
		Operations       []string `json:"operations"`
	}
	var grants []grantView

	paginator := kms.NewListGrantsPaginator(client, &kms.ListGrantsInput{KeyId: aws.String(keyID)})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, g := range page.Grants {
			ops := make([]string, 0, len(g.Operations))
			for _, o := range g.Operations {
				ops = append(ops, string(o))
			}
			grants = append(grants, grantView{
				GrantID:          aws.ToString(g.GrantId),
				GranteePrincipal: aws.ToString(g.GranteePrincipal),
				Operations:       ops,
			})
		}
	}

	grantsJSON, err := json.Marshal(grants)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d grant(s) on %s", len(grants), keyID),
		"grants":      string(grantsJSON),
		"count":       len(grants),
	}, nil
}
