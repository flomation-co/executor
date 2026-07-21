// Package aws_rds_describe_certificates lists the CA certificates available for
// RDS DB instances in the account.
package aws_rds_describe_certificates

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Describe Certificates"
	Description  = "List the CA certificates available for RDS DB instances."
	Website      = "https://www.flomation.co"
	Icon         = "lock+magnifying-glass"
	Date         = "20/07/2026"
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
	{Name: "certificate_identifier", Type: core.ConnectionTypeString, Label: "Certificate Identifier (optional)", Placeholder: "Leave blank to list all"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "certificates", Type: core.ConnectionTypeObject, Label: "Certificates"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.DescribeCertificatesInput{}
	if cid := awscommon.InputString("certificate_identifier", inputs); cid != "" {
		in.CertificateIdentifier = aws.String(cid)
	}

	var certificates []map[string]interface{}
	paginator := rds.NewDescribeCertificatesPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for i := range page.Certificates {
			c := &page.Certificates[i]
			m := map[string]interface{}{
				"certificate_identifier": aws.ToString(c.CertificateIdentifier),
				"certificate_type":       aws.ToString(c.CertificateType),
				"thumbprint":             aws.ToString(c.Thumbprint),
			}
			if c.ValidFrom != nil {
				m["valid_from"] = c.ValidFrom.UTC().Format("2006-01-02T15:04:05Z")
			}
			if c.ValidTill != nil {
				m["valid_till"] = c.ValidTill.UTC().Format("2006-01-02T15:04:05Z")
			}
			certificates = append(certificates, m)
		}
	}

	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Found %d certificate(s)", len(certificates)),
		"certificates": certificates,
		"count":        len(certificates),
	}, nil
}
