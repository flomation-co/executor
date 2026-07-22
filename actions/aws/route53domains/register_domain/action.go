// Package aws_route53domains_register_domain registers a new domain with the
// Route 53 registrar.
package aws_route53domains_register_domain

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
	Name         = "AWS Route 53 Register Domain"
	Description  = "Register a domain. Admin/registrant/tech contacts are ContactDetail-shaped JSON."
	Website      = "https://www.flomation.co"
	Icon         = "id-badge+plus"
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
	{Name: "duration_years", Type: core.ConnectionTypeInteger, Label: "Duration (years)", Placeholder: "1"},
	{Name: "auto_renew", Type: core.ConnectionTypeBoolean, Label: "Auto Renew"},
	{Name: "admin_contact", Type: core.ConnectionTypeString, Label: "Admin Contact (ContactDetail JSON)", Placeholder: `{"FirstName":"Jane","LastName":"Doe","ContactType":"PERSON","Email":"jane@example.com","CountryCode":"GB","AddressLine1":"1 High St","City":"London","ZipCode":"SW1A 1AA","PhoneNumber":"+44.2071234567"}`, Required: true},
	{Name: "registrant_contact", Type: core.ConnectionTypeString, Label: "Registrant Contact (ContactDetail JSON)", Required: true},
	{Name: "tech_contact", Type: core.ConnectionTypeString, Label: "Tech Contact (ContactDetail JSON)", Required: true},
	{Name: "privacy_protect_admin_contact", Type: core.ConnectionTypeBoolean, Label: "Privacy Protect Admin Contact"},
	{Name: "privacy_protect_registrant_contact", Type: core.ConnectionTypeBoolean, Label: "Privacy Protect Registrant Contact"},
	{Name: "privacy_protect_tech_contact", Type: core.ConnectionTypeBoolean, Label: "Privacy Protect Tech Contact"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "operation_id", Type: core.ConnectionTypeString, Label: "Operation ID"},
}

func parseContact(name string, inputs []*core.Connection) (*r53dtypes.ContactDetail, error) {
	raw := strings.TrimSpace(awscommon.InputString(name, inputs))
	if raw == "" {
		return nil, fmt.Errorf("%s is required (ContactDetail JSON)", name)
	}
	var c r53dtypes.ContactDetail
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		return nil, fmt.Errorf("%s must be ContactDetail JSON: %w", name, err)
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

	duration := int32(1)
	if n, ok := awscommon.InputInt("duration_years", inputs); ok {
		duration = int32(n)
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := route53domains.NewFromConfig(cfg, func(o *route53domains.Options) { o.Region = "us-east-1" })

	in := &route53domains.RegisterDomainInput{
		DomainName:        aws.String(domainName),
		DurationInYears:   aws.Int32(duration),
		AdminContact:      admin,
		RegistrantContact: registrant,
		TechContact:       tech,
		AutoRenew:         aws.Bool(awscommon.InputBool("auto_renew", inputs)),
	}
	if awscommon.InputBool("privacy_protect_admin_contact", inputs) {
		in.PrivacyProtectAdminContact = aws.Bool(true)
	}
	if awscommon.InputBool("privacy_protect_registrant_contact", inputs) {
		in.PrivacyProtectRegistrantContact = aws.Bool(true)
	}
	if awscommon.InputBool("privacy_protect_tech_contact", inputs) {
		in.PrivacyProtectTechContact = aws.Bool(true)
	}

	out, err := client.RegisterDomain(ctx, in)
	if err != nil {
		return nil, err
	}

	operationID := aws.ToString(out.OperationId)
	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Registration of %s started (operation %s)", domainName, operationID),
		"operation_id": operationID,
	}, nil
}
