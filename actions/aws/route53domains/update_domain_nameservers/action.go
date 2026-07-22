// Package aws_route53domains_update_domain_nameservers replaces the set of name
// servers for a registered domain.
package aws_route53domains_update_domain_nameservers

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
	Name         = "AWS Route 53 Update Nameservers"
	Description  = "Replace the set of name servers for a registered domain."
	Website      = "https://www.flomation.co"
	Icon         = "id-badge+network-wired"
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
	{Name: "nameservers", Type: core.ConnectionTypeString, Label: "Nameservers (JSON array or comma-separated hosts)", Placeholder: `ns-1.example.com,ns-2.example.com  OR  [{"Name":"ns-1.example.com","GlueIps":["192.0.2.1"]}]`, Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "operation_id", Type: core.ConnectionTypeString, Label: "Operation ID"},
}

// parseNameservers accepts either a JSON array of {Name, GlueIps?} objects or a
// simple comma-separated list of host names.
func parseNameservers(raw string) ([]r53dtypes.Nameserver, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("nameservers are required")
	}
	if strings.HasPrefix(raw, "[") {
		var ns []r53dtypes.Nameserver
		if err := json.Unmarshal([]byte(raw), &ns); err != nil {
			return nil, fmt.Errorf("nameservers must be a JSON array of {Name, GlueIps?}: %w", err)
		}
		if len(ns) == 0 {
			return nil, fmt.Errorf("at least one nameserver is required")
		}
		return ns, nil
	}
	var ns []r53dtypes.Nameserver
	for _, host := range strings.Split(raw, ",") {
		host = strings.TrimSpace(host)
		if host == "" {
			continue
		}
		ns = append(ns, r53dtypes.Nameserver{Name: aws.String(host)})
	}
	if len(ns) == 0 {
		return nil, fmt.Errorf("at least one nameserver is required")
	}
	return ns, nil
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	domainName := strings.TrimSpace(awscommon.InputString("domain_name", inputs))
	if domainName == "" {
		return nil, fmt.Errorf("domain name is required")
	}

	nameservers, err := parseNameservers(awscommon.InputString("nameservers", inputs))
	if err != nil {
		return nil, err
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := route53domains.NewFromConfig(cfg, func(o *route53domains.Options) {
		o.Region = "us-east-1"
	})

	out, err := client.UpdateDomainNameservers(ctx, &route53domains.UpdateDomainNameserversInput{
		DomainName:  aws.String(domainName),
		Nameservers: nameservers,
	})
	if err != nil {
		return nil, err
	}

	operationID := aws.ToString(out.OperationId)
	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Requested nameserver update for %s with %d server(s) (operation %s)", domainName, len(nameservers), operationID),
		"operation_id": operationID,
	}, nil
}
