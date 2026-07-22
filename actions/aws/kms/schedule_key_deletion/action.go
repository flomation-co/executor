// Package aws_kms_schedule_key_deletion schedules deletion of an AWS KMS key.
package aws_kms_schedule_key_deletion

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS KMS Schedule Key Deletion"
	Description  = "Schedule an AWS KMS key for deletion after a waiting period."
	Website      = "https://www.flomation.co"
	Icon         = "key+trash"
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
	{Name: "key_id", Type: core.ConnectionTypeString, Label: "Key ID, ARN or Alias", Placeholder: "1234abcd-... / alias/my-key", Required: true},
	{Name: "pending_window_in_days", Type: core.ConnectionTypeInteger, Label: "Pending Window (days, 7-30)", Placeholder: "30"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "key_id", Type: core.ConnectionTypeString, Label: "Key ID"},
	{Name: "deletion_date", Type: core.ConnectionTypeString, Label: "Deletion Date"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	keyID := awscommon.InputString("key_id", inputs)
	if keyID == "" {
		return nil, fmt.Errorf("key id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := kms.NewFromConfig(cfg)

	in := &kms.ScheduleKeyDeletionInput{KeyId: aws.String(keyID)}
	if n, ok := awscommon.InputInt("pending_window_in_days", inputs); ok {
		in.PendingWindowInDays = aws.Int32(int32(n))
	}

	out, err := client.ScheduleKeyDeletion(ctx, in)
	if err != nil {
		return nil, err
	}

	var deletionDate string
	if out.DeletionDate != nil {
		deletionDate = out.DeletionDate.Format("2006-01-02T15:04:05Z07:00")
	}
	resolvedID := aws.ToString(out.KeyId)
	if resolvedID == "" {
		resolvedID = keyID
	}

	return map[string]interface{}{
		"tool_result":   fmt.Sprintf("Scheduled KMS key %s for deletion on %s", resolvedID, deletionDate),
		"key_id":        resolvedID,
		"deletion_date": deletionDate,
	}, nil
}
