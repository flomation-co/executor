// Package aws_route53_deactivate_key_signing_key deactivates a key-signing key in a Route 53 hosted zone.
package aws_route53_deactivate_key_signing_key

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Route 53 Deactivate Key Signing Key"
	Description  = "Deactivate a DNSSEC key-signing key (KSK) in a hosted zone."
	Website      = "https://www.flomation.co"
	Icon         = "globe+trash"
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
	{Name: "name", Type: core.ConnectionTypeString, Label: "Key Signing Key Name", Placeholder: "my_ksk", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "change_id", Type: core.ConnectionTypeString, Label: "Change ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	zoneID := awscommon.InputString("hosted_zone_id", inputs)
	if zoneID == "" {
		return nil, fmt.Errorf("hosted zone id is required")
	}
	name := awscommon.InputString("name", inputs)
	if name == "" {
		return nil, fmt.Errorf("key signing key name is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := route53.NewFromConfig(cfg)

	out, err := client.DeactivateKeySigningKey(ctx, &route53.DeactivateKeySigningKeyInput{
		HostedZoneId: aws.String(zoneID),
		Name:         aws.String(name),
	})
	if err != nil {
		return nil, err
	}

	changeID := ""
	if out.ChangeInfo != nil {
		changeID = aws.ToString(out.ChangeInfo.Id)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Deactivated key-signing key %s in hosted zone %s", name, zoneID),
		"change_id":   changeID,
	}, nil
}
