// Package aws_s3_put_account_public_access_block sets the account-level S3
// public access block configuration (via the s3control SDK).
package aws_s3_put_account_public_access_block

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3control"
	s3ctltypes "github.com/aws/aws-sdk-go-v2/service/s3control/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS S3 Put Account Public Access Block"
	Description  = "Set the account-level S3 public access block configuration."
	Website      = "https://www.flomation.co"
	Icon         = "shield-halved+pen"
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
	{Name: "block_public_acls", Type: core.ConnectionTypeBoolean, Label: "Block Public ACLs", Value: true},
	{Name: "ignore_public_acls", Type: core.ConnectionTypeBoolean, Label: "Ignore Public ACLs", Value: true},
	{Name: "block_public_policy", Type: core.ConnectionTypeBoolean, Label: "Block Public Policy", Value: true},
	{Name: "restrict_public_buckets", Type: core.ConnectionTypeBoolean, Label: "Restrict Public Buckets", Value: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Account ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	blockACLs := awscommon.InputBool("block_public_acls", inputs)
	ignoreACLs := awscommon.InputBool("ignore_public_acls", inputs)
	blockPolicy := awscommon.InputBool("block_public_policy", inputs)
	restrict := awscommon.InputBool("restrict_public_buckets", inputs)

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}

	accountID, err := awscommon.ResolveAccountID(ctx, cfg, inputs)
	if err != nil {
		return nil, err
	}

	client := s3control.NewFromConfig(cfg)

	_, err = client.PutPublicAccessBlock(ctx, &s3control.PutPublicAccessBlockInput{
		AccountId: aws.String(accountID),
		PublicAccessBlockConfiguration: &s3ctltypes.PublicAccessBlockConfiguration{
			BlockPublicAcls:       aws.Bool(blockACLs),
			IgnorePublicAcls:      aws.Bool(ignoreACLs),
			BlockPublicPolicy:     aws.Bool(blockPolicy),
			RestrictPublicBuckets: aws.Bool(restrict),
		},
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Updated account-level public access block for account %s", accountID),
		"account_id":  accountID,
	}, nil
}
