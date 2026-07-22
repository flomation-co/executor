// Package aws_route53domains_update_domain_contact updates the admin,
// registrant, and/or technical contact details for a registered domain.
package aws_route53domains_update_domain_contact

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53domains"
	r53dtypes "github.com/aws/aws-sdk-go-v2/service/route53domains/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Route 53 Update Domain Contact"
	Description  = "Update admin, registrant and/or tech contact details for a registered domain."
	Website      = "https://www.flomation.co"
	Icon         = "id-badge+pen"
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
	{Name: "admin_contact", Type: core.ConnectionTypeString, Label: "Admin Contact (JSON)", Placeholder: `{"FirstName":"Jane","LastName":"Doe","Email":"jane@example.com"}`},
	{Name: "registrant_contact", Type: core.ConnectionTypeString, Label: "Registrant Contact (JSON)", Placeholder: `{"FirstName":"Jane","LastName":"Doe","Email":"jane@example.com"}`},
	{Name: "tech_contact", Type: core.ConnectionTypeString, Label: "Tech Contact (JSON)", Placeholder: `{"FirstName":"Jane","LastName":"Doe","Email":"jane@example.com"}`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "operation_id", Type: core.ConnectionTypeString, Label: "Operation ID"},
}

// parseContact unmarshals an optional ContactDetail JSON input. Empty input
// returns (nil, nil) so the contact is left untouched.
func parseContact(name string, inputs []*core.Connection) (*r53dtypes.ContactDetail, error) {
	raw := strings.TrimSpace(awscommon.InputString(name, inputs))
	if raw == "" {
		return nil, nil
	}
	var c r53dtypes.ContactDetail
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return nil, fmt.Errorf("%s must be a JSON object: %w", name, err)
	}
	return &c, nil
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	domainName := strings.TrimSpace(awscommon.InputString("domain_name", inputs))
	if domainName == "" {
		return nil, fmt.Errorf("domain name is required")
	}

	admin, err := parseContact("admin_contact", inputs)
	if err != nil {
		return nil, err
	}
	registrant, err := parseContact("registrant_contact", inputs)
	if err != nil {
		return nil, err
	}
	tech, err := parseContact("tech_contact", inputs)
	if err != nil {
		return nil, err
	}
	if admin == nil && registrant == nil && tech == nil {
		return nil, fmt.Errorf("provide at least one of admin_contact, registrant_contact or tech_contact")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := route53domains.NewFromConfig(cfg, func(o *route53domains.Options) {
		o.Region = "us-east-1"
	})

	in := &route53domains.UpdateDomainContactInput{DomainName: aws.String(domainName)}
	if admin != nil {
		in.AdminContact = admin
	}
	if registrant != nil {
		in.RegistrantContact = registrant
	}
	if tech != nil {
		in.TechContact = tech
	}

	out, err := client.UpdateDomainContact(ctx, in)
	if err != nil {
		return nil, err
	}

	operationID := aws.ToString(out.OperationId)
	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Requested contact update for %s (operation %s)", domainName, operationID),
		"operation_id": operationID,
	}, nil
}
