// Package aws_rds_modify_certificates overrides or resets the account-default RDS
// CA certificate.
package aws_rds_modify_certificates

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Modify Certificates"
	Description  = "Override or reset the account-default RDS CA certificate."
	Website      = "https://www.flomation.co"
	Icon         = "lock+pen"
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
	{Name: "certificate_identifier", Type: core.ConnectionTypeString, Label: "Certificate Identifier (optional)", Placeholder: "rds-ca-rsa2048-g1"},
	{Name: "remove_customer_override", Type: core.ConnectionTypeBoolean, Label: "Remove Customer Override"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "certificate", Type: core.ConnectionTypeObject, Label: "Certificate"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.ModifyCertificatesInput{}
	if certID := awscommon.InputString("certificate_identifier", inputs); certID != "" {
		in.CertificateIdentifier = aws.String(certID)
	}
	if awscommon.InputBool("remove_customer_override", inputs) {
		in.RemoveCustomerOverride = aws.Bool(true)
	}

	out, err := client.ModifyCertificates(ctx, in)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Modified default CA certificate to %q", aws.ToString(out.Certificate.CertificateIdentifier)),
		"certificate": flattenCertificate(out.Certificate),
	}, nil
}

func flattenCertificate(c *rdstypes.Certificate) map[string]interface{} {
	if c == nil {
		return nil
	}
	var validFrom, validTill string
	if c.ValidFrom != nil {
		validFrom = c.ValidFrom.Format("2006-01-02T15:04:05Z07:00")
	}
	if c.ValidTill != nil {
		validTill = c.ValidTill.Format("2006-01-02T15:04:05Z07:00")
	}
	return map[string]interface{}{
		"certificate_identifier": aws.ToString(c.CertificateIdentifier),
		"certificate_type":       aws.ToString(c.CertificateType),
		"valid_from":             validFrom,
		"valid_till":             validTill,
	}
}
