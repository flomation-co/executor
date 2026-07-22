// Package aws_route53domains_disassociate_delegation_signer_from_domain removes
// a DNSSEC delegation signer (DS) record from a domain.
package aws_route53domains_disassociate_delegation_signer_from_domain

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53domains"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Route 53 Disassociate Delegation Signer From Domain"
	Description  = "Remove a DNSSEC delegation signer (DS) record from a domain by its ID."
	Website      = "https://www.flomation.co"
	Icon         = "id-badge+lock"
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
	{Name: "domain_name", Type: core.ConnectionTypeString, Label: "Domain Name", Placeholder: "example.com", Required: true},
	{Name: "id", Type: core.ConnectionTypeString, Label: "DS Record ID", Placeholder: "The internal ID of the DS record", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "operation_id", Type: core.ConnectionTypeString, Label: "Operation ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	domainName := strings.TrimSpace(awscommon.InputString("domain_name", inputs))
	if domainName == "" {
		return nil, fmt.Errorf("domain name is required")
	}
	id := strings.TrimSpace(awscommon.InputString("id", inputs))
	if id == "" {
		return nil, fmt.Errorf("DS record ID is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := route53domains.NewFromConfig(cfg, func(o *route53domains.Options) { o.Region = "us-east-1" })

	out, err := client.DisassociateDelegationSignerFromDomain(ctx, &route53domains.DisassociateDelegationSignerFromDomainInput{
		DomainName: aws.String(domainName),
		Id:         aws.String(id),
	})
	if err != nil {
		return nil, err
	}

	operationID := aws.ToString(out.OperationId)
	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Disassociated delegation signer %s from %s (operation %s)", id, domainName, operationID),
		"operation_id": operationID,
	}, nil
}
