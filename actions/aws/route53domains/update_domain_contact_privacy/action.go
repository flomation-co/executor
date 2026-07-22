// Package aws_route53domains_update_domain_contact_privacy toggles WHOIS
// privacy protection for a domain's admin, registrant and tech contacts.
package aws_route53domains_update_domain_contact_privacy

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
	Name         = "AWS Route 53 Update Contact Privacy"
	Description  = "Toggle WHOIS privacy for a domain's admin, registrant and tech contacts."
	Website      = "https://www.flomation.co"
	Icon         = "id-badge+shield-halved"
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
	{Name: "admin_privacy", Type: core.ConnectionTypeBoolean, Label: "Admin Privacy"},
	{Name: "registrant_privacy", Type: core.ConnectionTypeBoolean, Label: "Registrant Privacy"},
	{Name: "tech_privacy", Type: core.ConnectionTypeBoolean, Label: "Tech Privacy"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "operation_id", Type: core.ConnectionTypeString, Label: "Operation ID"},
}

// present reports whether an input carries a non-empty value, so a privacy flag
// is only sent to AWS when the user actually set it (AWS requires the same value
// across all contacts, so unset flags must be omitted, not defaulted to false).
func present(name string, inputs []*core.Connection) bool {
	c := core.FindConnection(name, inputs)
	if c == nil || c.Value == nil {
		return false
	}
	if b := c.Boolean(); b != nil {
		return true
	}
	return strings.TrimSpace(awscommon.InputString(name, inputs)) != ""
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	domainName := strings.TrimSpace(awscommon.InputString("domain_name", inputs))
	if domainName == "" {
		return nil, fmt.Errorf("domain name is required")
	}

	if !present("admin_privacy", inputs) && !present("registrant_privacy", inputs) && !present("tech_privacy", inputs) {
		return nil, fmt.Errorf("provide at least one of admin_privacy, registrant_privacy or tech_privacy")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := route53domains.NewFromConfig(cfg, func(o *route53domains.Options) {
		o.Region = "us-east-1"
	})

	in := &route53domains.UpdateDomainContactPrivacyInput{DomainName: aws.String(domainName)}
	if present("admin_privacy", inputs) {
		in.AdminPrivacy = aws.Bool(awscommon.InputBool("admin_privacy", inputs))
	}
	if present("registrant_privacy", inputs) {
		in.RegistrantPrivacy = aws.Bool(awscommon.InputBool("registrant_privacy", inputs))
	}
	if present("tech_privacy", inputs) {
		in.TechPrivacy = aws.Bool(awscommon.InputBool("tech_privacy", inputs))
	}

	out, err := client.UpdateDomainContactPrivacy(ctx, in)
	if err != nil {
		return nil, err
	}

	operationID := aws.ToString(out.OperationId)
	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Requested privacy update for %s (operation %s)", domainName, operationID),
		"operation_id": operationID,
	}, nil
}
