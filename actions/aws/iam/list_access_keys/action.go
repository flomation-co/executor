// Package aws_iam_list_access_keys lists an IAM user's access keys.
package aws_iam_list_access_keys

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS IAM List Access Keys"
	Description  = "List access keys for an IAM user (defaults to the calling user)."
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
	{Name: "user_name", Type: core.ConnectionTypeString, Label: "User Name (optional)", Placeholder: "jane.doe"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "access_keys", Type: core.ConnectionTypeString, Label: "Access Keys (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := iam.NewFromConfig(cfg)

	in := &iam.ListAccessKeysInput{}
	if u := awscommon.InputString("user_name", inputs); u != "" {
		in.UserName = aws.String(u)
	}

	out, err := client.ListAccessKeys(ctx, in)
	if err != nil {
		return nil, err
	}

	type keyRow struct {
		AccessKeyID string `json:"access_key_id"`
		Status      string `json:"status"`
		CreateDate  string `json:"create_date"`
	}
	rows := make([]keyRow, 0, len(out.AccessKeyMetadata))
	for _, k := range out.AccessKeyMetadata {
		var created string
		if k.CreateDate != nil {
			created = k.CreateDate.Format(time.RFC3339)
		}
		rows = append(rows, keyRow{
			AccessKeyID: aws.ToString(k.AccessKeyId),
			Status:      string(k.Status),
			CreateDate:  created,
		})
	}

	keysJSON, err := json.Marshal(rows)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d access key(s)", len(rows)),
		"access_keys": string(keysJSON),
		"count":       len(rows),
	}, nil
}
