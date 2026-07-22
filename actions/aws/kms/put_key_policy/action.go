// Package aws_kms_put_key_policy attaches a key policy to a KMS key.
package aws_kms_put_key_policy

import (
	"context"
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
	Name         = "AWS KMS Put Key Policy"
	Description  = "Attach or replace the key policy document on a KMS key."
	Website      = "https://www.flomation.co"
	Icon         = "key+file-lines"
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
	{Name: "key_id", Type: core.ConnectionTypeString, Label: "Key ID / ARN", Placeholder: "1234abcd-12ab-34cd-56ef-1234567890ab", Required: true},
	{Name: "policy", Type: core.ConnectionTypeString, Label: "Policy Document (JSON)", Placeholder: `{"Version":"2012-10-17","Statement":[...]}`, Required: true},
	{Name: "policy_name", Type: core.ConnectionTypeString, Label: "Policy Name (optional)", Placeholder: "default"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "key_id", Type: core.ConnectionTypeString, Label: "Key ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	keyID := awscommon.InputString("key_id", inputs)
	if keyID == "" {
		return nil, fmt.Errorf("key id is required")
	}
	policy := awscommon.InputString("policy", inputs)
	if strings.TrimSpace(policy) == "" {
		return nil, fmt.Errorf("policy is required")
	}
	policyName := strings.TrimSpace(awscommon.InputString("policy_name", inputs))
	if policyName == "" {
		policyName = "default"
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := kms.NewFromConfig(cfg)

	_, err = client.PutKeyPolicy(ctx, &kms.PutKeyPolicyInput{
		KeyId:      aws.String(keyID),
		Policy:     aws.String(policy),
		PolicyName: aws.String(policyName),
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": "Updated key policy for " + keyID,
		"key_id":      keyID,
	}, nil
}
