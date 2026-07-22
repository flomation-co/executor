// Package aws_kms_create_grant creates a grant on a KMS key.
package aws_kms_create_grant

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS KMS Create Grant"
	Description  = "Grant a principal permission to use a KMS key for specific operations."
	Website      = "https://www.flomation.co"
	Icon         = "key+plus"
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
	{Name: "grantee_principal", Type: core.ConnectionTypeString, Label: "Grantee Principal ARN", Placeholder: "arn:aws:iam::123456789012:role/my-role", Required: true},
	{Name: "operations", Type: core.ConnectionTypeString, Label: "Operations (comma-separated)", Placeholder: "Decrypt,Encrypt,GenerateDataKey", Required: true},
	{Name: "retiring_principal", Type: core.ConnectionTypeString, Label: "Retiring Principal ARN (optional)", Placeholder: "arn:aws:iam::123456789012:role/retirer"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Grant Name (optional)", Placeholder: "my-grant"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "grant_id", Type: core.ConnectionTypeString, Label: "Grant ID"},
	{Name: "grant_token", Type: core.ConnectionTypeString, Label: "Grant Token"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	keyID := awscommon.InputString("key_id", inputs)
	if keyID == "" {
		return nil, fmt.Errorf("key_id is required")
	}
	grantee := awscommon.InputString("grantee_principal", inputs)
	if grantee == "" {
		return nil, fmt.Errorf("grantee_principal is required")
	}
	opStrings := awscommon.InputStrings("operations", inputs)
	if len(opStrings) == 0 {
		return nil, fmt.Errorf("operations is required")
	}

	operations := make([]kmstypes.GrantOperation, 0, len(opStrings))
	for _, o := range opStrings {
		operations = append(operations, kmstypes.GrantOperation(o))
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := kms.NewFromConfig(cfg)

	in := &kms.CreateGrantInput{
		KeyId:            aws.String(keyID),
		GranteePrincipal: aws.String(grantee),
		Operations:       operations,
	}
	if rp := awscommon.InputString("retiring_principal", inputs); rp != "" {
		in.RetiringPrincipal = aws.String(rp)
	}
	if name := awscommon.InputString("name", inputs); name != "" {
		in.Name = aws.String(name)
	}

	out, err := client.CreateGrant(ctx, in)
	if err != nil {
		return nil, err
	}

	grantID := aws.ToString(out.GrantId)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created grant %s on %s", grantID, keyID),
		"grant_id":    grantID,
		"grant_token": aws.ToString(out.GrantToken),
	}, nil
}
