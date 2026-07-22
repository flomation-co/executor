// Package aws_route53_list_hosted_zones_by_name lists Route 53 hosted zones ordered by name.
package aws_route53_list_hosted_zones_by_name

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Route 53 List Hosted Zones By Name"
	Description  = "List Route 53 hosted zones ordered by name, optionally filtered by DNS name."
	Website      = "https://www.flomation.co"
	Icon         = "globe+magnifying-glass"
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
	{Name: "dns_name", Type: core.ConnectionTypeString, Label: "DNS Name Filter", Placeholder: "example.com (optional prefix)"},
	{Name: "max_items", Type: core.ConnectionTypeInteger, Label: "Max Items", Placeholder: "Optional"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "hosted_zones", Type: core.ConnectionTypeString, Label: "Hosted Zones"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := route53.NewFromConfig(cfg)

	in := &route53.ListHostedZonesByNameInput{}
	if dnsName := awscommon.InputString("dns_name", inputs); dnsName != "" {
		in.DNSName = aws.String(dnsName)
	}
	if n, ok := awscommon.InputInt("max_items", inputs); ok {
		in.MaxItems = aws.Int32(int32(n))
	}

	out, err := client.ListHostedZonesByName(ctx, in)
	if err != nil {
		return nil, err
	}

	zones := []map[string]interface{}{}
	for _, hz := range out.HostedZones {
		private := false
		if hz.Config != nil {
			private = hz.Config.PrivateZone
		}
		zones = append(zones, map[string]interface{}{
			"id":           aws.ToString(hz.Id),
			"name":         aws.ToString(hz.Name),
			"record_count": aws.ToInt64(hz.ResourceRecordSetCount),
			"private":      private,
		})
	}

	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Found %d hosted zone(s)", len(zones)),
		"count":        int64(len(zones)),
		"hosted_zones": zones,
	}, nil
}
