// Package aws_route53_create_key_signing_key creates a key-signing key for a Route 53 hosted zone.
package aws_route53_create_key_signing_key

import (
	"context"
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Route 53 Create Key Signing Key"
	Description  = "Create a DNSSEC key-signing key (KSK) for a hosted zone using a KMS key."
	Website      = "https://www.flomation.co"
	Icon         = "globe+key"
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
	{Name: "hosted_zone_id", Type: core.ConnectionTypeString, Label: "Hosted Zone ID", Placeholder: "Z1234567890ABC", Required: true},
	{Name: "key_management_service_arn", Type: core.ConnectionTypeString, Label: "KMS Key ARN", Placeholder: "arn:aws:kms:us-east-1:111122223333:key/...", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Key Signing Key Name", Placeholder: "my_ksk", Required: true},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Initial Status", Required: true, Options: []core.ConnectionOption{
		{Name: "Active", Value: "ACTIVE"},
		{Name: "Inactive", Value: "INACTIVE"},
	}},
	{Name: "caller_reference", Type: core.ConnectionTypeString, Label: "Caller Reference", Placeholder: "Optional — a unique idempotency token"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "key_signing_key", Type: core.ConnectionTypeString, Label: "Key Signing Key (JSON)"},
	{Name: "change_id", Type: core.ConnectionTypeString, Label: "Change ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	zoneID := awscommon.InputString("hosted_zone_id", inputs)
	if zoneID == "" {
		return nil, fmt.Errorf("hosted zone id is required")
	}
	kmsArn := awscommon.InputString("key_management_service_arn", inputs)
	if kmsArn == "" {
		return nil, fmt.Errorf("KMS key ARN is required")
	}
	name := awscommon.InputString("name", inputs)
	if name == "" {
		return nil, fmt.Errorf("key signing key name is required")
	}

	status := awscommon.InputString("status", inputs)
	if status == "" {
		status = "ACTIVE"
	}

	callerRef := awscommon.InputString("caller_reference", inputs)
	if callerRef == "" {
		// CallerReference must be non-empty and unique; derive a stable value from
		// the name so a retried request stays idempotent.
		callerRef = "flomation-" + name
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := route53.NewFromConfig(cfg)

	out, err := client.CreateKeySigningKey(ctx, &route53.CreateKeySigningKeyInput{
		HostedZoneId:            aws.String(zoneID),
		KeyManagementServiceArn: aws.String(kmsArn),
		Name:                    aws.String(name),
		Status:                  aws.String(status),
		CallerReference:         aws.String(callerRef),
	})
	if err != nil {
		return nil, err
	}

	kskJSON, _ := json.Marshal(out.KeySigningKey)

	changeID := ""
	if out.ChangeInfo != nil {
		changeID = aws.ToString(out.ChangeInfo.Id)
	}

	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Created key-signing key %s for hosted zone %s", name, zoneID),
		"key_signing_key": string(kskJSON),
		"change_id":       changeID,
	}, nil
}
