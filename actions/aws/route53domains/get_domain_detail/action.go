// Package aws_route53domains_get_domain_detail fetches registration detail for a
// domain.
package aws_route53domains_get_domain_detail

import (
	"context"
	"encoding/json"
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
	Name         = "AWS Route 53 Get Domain Detail"
	Description  = "Fetch a domain's registration detail: nameservers, expiry and registrant."
	Website      = "https://www.flomation.co"
	Icon         = "id-badge+magnifying-glass"
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "domain_name", Type: core.ConnectionTypeString, Label: "Domain Name"},
	{Name: "nameservers", Type: core.ConnectionTypeString, Label: "Nameservers (JSON)"},
	{Name: "auto_renew", Type: core.ConnectionTypeBoolean, Label: "Auto Renew"},
	{Name: "expiry", Type: core.ConnectionTypeString, Label: "Expiry"},
	{Name: "registrant_contact", Type: core.ConnectionTypeString, Label: "Registrant Contact (JSON)"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	domainName := strings.TrimSpace(awscommon.InputString("domain_name", inputs))
	if domainName == "" {
		return nil, fmt.Errorf("domain name is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := route53domains.NewFromConfig(cfg, func(o *route53domains.Options) { o.Region = "us-east-1" })

	out, err := client.GetDomainDetail(ctx, &route53domains.GetDomainDetailInput{
		DomainName: aws.String(domainName),
	})
	if err != nil {
		return nil, err
	}

	nameservers := make([]string, 0, len(out.Nameservers))
	for _, ns := range out.Nameservers {
		nameservers = append(nameservers, aws.ToString(ns.Name))
	}
	nsJSON, err := json.Marshal(nameservers)
	if err != nil {
		return nil, fmt.Errorf("encode nameservers: %w", err)
	}

	registrantJSON, err := json.Marshal(out.RegistrantContact)
	if err != nil {
		return nil, fmt.Errorf("encode registrant contact: %w", err)
	}

	expiry := ""
	if out.ExpirationDate != nil {
		expiry = out.ExpirationDate.UTC().Format("2006-01-02T15:04:05Z")
	}

	return map[string]interface{}{
		"tool_result":        fmt.Sprintf("%s expires %s", domainName, expiry),
		"domain_name":        aws.ToString(out.DomainName),
		"nameservers":        string(nsJSON),
		"auto_renew":         aws.ToBool(out.AutoRenew),
		"expiry":             expiry,
		"registrant_contact": string(registrantJSON),
	}, nil
}
