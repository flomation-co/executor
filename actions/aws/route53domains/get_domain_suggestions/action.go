// Package aws_route53domains_get_domain_suggestions suggests available domain
// names based on a seed name.
package aws_route53domains_get_domain_suggestions

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
	Name         = "AWS Route 53 Get Domain Suggestions"
	Description  = "Suggest domain names based on a seed name, optionally only available ones."
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
	{Name: "suggestion_count", Type: core.ConnectionTypeInteger, Label: "Suggestion Count", Placeholder: "10"},
	{Name: "only_available", Type: core.ConnectionTypeBoolean, Label: "Only Available"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "suggestions", Type: core.ConnectionTypeString, Label: "Suggestions (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Suggestion Count"},
}

type suggestionRow struct {
	DomainName   string `json:"domain_name"`
	Availability string `json:"availability"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	domainName := strings.TrimSpace(awscommon.InputString("domain_name", inputs))
	if domainName == "" {
		return nil, fmt.Errorf("domain name is required")
	}

	count := int32(10)
	if n, ok := awscommon.InputInt("suggestion_count", inputs); ok {
		count = int32(n)
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := route53domains.NewFromConfig(cfg, func(o *route53domains.Options) { o.Region = "us-east-1" })

	out, err := client.GetDomainSuggestions(ctx, &route53domains.GetDomainSuggestionsInput{
		DomainName:      aws.String(domainName),
		SuggestionCount: count,
		OnlyAvailable:   aws.Bool(awscommon.InputBool("only_available", inputs)),
	})
	if err != nil {
		return nil, err
	}

	rows := make([]suggestionRow, 0, len(out.SuggestionsList))
	for _, s := range out.SuggestionsList {
		rows = append(rows, suggestionRow{
			DomainName:   aws.ToString(s.DomainName),
			Availability: aws.ToString(s.Availability),
		})
	}

	suggestionsJSON, err := json.Marshal(rows)
	if err != nil {
		return nil, fmt.Errorf("encode suggestions: %w", err)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d suggestion(s) for %s", len(rows), domainName),
		"suggestions": string(suggestionsJSON),
		"count":       len(rows),
	}, nil
}
