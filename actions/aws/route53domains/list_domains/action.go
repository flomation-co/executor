// Package aws_route53domains_list_domains lists the domains registered in the
// account.
package aws_route53domains_list_domains

import (
	"context"
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53domains"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Route 53 List Domains"
	Description  = "List the domains registered in the account with renewal and expiry details."
	Website      = "https://www.flomation.co"
	Icon         = "id-badge+list"
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "domains", Type: core.ConnectionTypeString, Label: "Domains (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Domain Count"},
}

type domainRow struct {
	DomainName string `json:"domain_name"`
	AutoRenew  bool   `json:"auto_renew"`
	Expiry     string `json:"expiry"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := route53domains.NewFromConfig(cfg, func(o *route53domains.Options) { o.Region = "us-east-1" })

	var rows []domainRow
	paginator := route53domains.NewListDomainsPaginator(client, &route53domains.ListDomainsInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, d := range page.Domains {
			row := domainRow{
				DomainName: aws.ToString(d.DomainName),
				AutoRenew:  aws.ToBool(d.AutoRenew),
			}
			if d.Expiry != nil {
				row.Expiry = d.Expiry.UTC().Format("2006-01-02T15:04:05Z")
			}
			rows = append(rows, row)
		}
	}

	domainsJSON, err := json.Marshal(rows)
	if err != nil {
		return nil, fmt.Errorf("encode domains: %w", err)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d domain(s)", len(rows)),
		"domains":     string(domainsJSON),
		"count":       len(rows),
	}, nil
}
