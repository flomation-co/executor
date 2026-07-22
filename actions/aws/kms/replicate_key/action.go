// Package aws_kms_replicate_key replicates a multi-region KMS key into another region.
package aws_kms_replicate_key

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
	Name         = "AWS KMS Replicate Key"
	Description  = "Replicate a multi-region KMS key into another AWS region."
	Website      = "https://www.flomation.co"
	Icon         = "key+copy"
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
	{Name: "key_id", Type: core.ConnectionTypeString, Label: "Multi-Region Key ID / ARN", Placeholder: "mrk-1234abcd...", Required: true},
	{Name: "replica_region", Type: core.ConnectionTypeString, Label: "Replica Region", Placeholder: "us-east-1", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description (optional)"},
	{Name: "policy", Type: core.ConnectionTypeString, Label: "Key Policy Document (JSON, optional)", Placeholder: `{"Version":"2012-10-17","Statement":[...]}`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "replica_key_id", Type: core.ConnectionTypeString, Label: "Replica Key ID"},
	{Name: "replica_arn", Type: core.ConnectionTypeString, Label: "Replica Key ARN"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	keyID := awscommon.InputString("key_id", inputs)
	if keyID == "" {
		return nil, fmt.Errorf("key id is required")
	}
	replicaRegion := awscommon.InputString("replica_region", inputs)
	if replicaRegion == "" {
		return nil, fmt.Errorf("replica region is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := kms.NewFromConfig(cfg)

	in := &kms.ReplicateKeyInput{
		KeyId:         aws.String(keyID),
		ReplicaRegion: aws.String(replicaRegion),
	}
	if d := strings.TrimSpace(awscommon.InputString("description", inputs)); d != "" {
		in.Description = aws.String(d)
	}
	if p := strings.TrimSpace(awscommon.InputString("policy", inputs)); p != "" {
		in.Policy = aws.String(p)
	}

	out, err := client.ReplicateKey(ctx, in)
	if err != nil {
		return nil, err
	}

	var replicaKeyID, replicaARN string
	if out.ReplicaKeyMetadata != nil {
		replicaKeyID = aws.ToString(out.ReplicaKeyMetadata.KeyId)
		replicaARN = aws.ToString(out.ReplicaKeyMetadata.Arn)
	}
	return map[string]interface{}{
		"tool_result":    fmt.Sprintf("Replicated key %s into %s", keyID, replicaRegion),
		"replica_key_id": replicaKeyID,
		"replica_arn":    replicaARN,
	}, nil
}
