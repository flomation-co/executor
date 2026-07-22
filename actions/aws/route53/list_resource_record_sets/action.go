// Package aws_route53_list_resource_record_sets lists DNS records in a
// Route 53 hosted zone.
package aws_route53_list_resource_record_sets

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Route 53 List Records"
	Description  = "List the DNS records in a Route 53 hosted zone."
	Website      = "https://www.flomation.co"
	Icon         = "route+magnifying-glass"
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
	{Name: "hosted_zone_id", Type: core.ConnectionTypeString, Label: "Hosted Zone ID", Placeholder: "Z1234567890ABC", Required: true},
	{Name: "start_record_name", Type: core.ConnectionTypeString, Label: "Start Record Name (optional)"},
	{Name: "start_record_type", Type: core.ConnectionTypeString, Label: "Start Record Type (optional)"},
	{Name: "max_items", Type: core.ConnectionTypeInteger, Label: "Max Items (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "records", Type: core.ConnectionTypeString, Label: "Records (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

type recordOut struct {
	Name   string   `json:"name"`
	Type   string   `json:"type"`
	TTL    int64    `json:"ttl"`
	Values []string `json:"values"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	zoneID := awscommon.InputString("hosted_zone_id", inputs)
	if zoneID == "" {
		return nil, fmt.Errorf("hosted zone id is required")
	}

	in := &route53.ListResourceRecordSetsInput{HostedZoneId: aws.String(zoneID)}
	if v := strings.TrimSpace(awscommon.InputString("start_record_name", inputs)); v != "" {
		in.StartRecordName = aws.String(v)
	}
	if v := strings.TrimSpace(awscommon.InputString("start_record_type", inputs)); v != "" {
		in.StartRecordType = r53types.RRType(v)
	}
	if v, ok := awscommon.InputInt("max_items", inputs); ok {
		in.MaxItems = aws.Int32(int32(v))
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := route53.NewFromConfig(cfg)

	out, err := client.ListResourceRecordSets(ctx, in)
	if err != nil {
		return nil, err
	}

	records := make([]recordOut, 0, len(out.ResourceRecordSets))
	for _, rs := range out.ResourceRecordSets {
		var values []string
		for _, r := range rs.ResourceRecords {
			values = append(values, aws.ToString(r.Value))
		}
		records = append(records, recordOut{
			Name:   aws.ToString(rs.Name),
			Type:   string(rs.Type),
			TTL:    aws.ToInt64(rs.TTL),
			Values: values,
		})
	}

	jsonBytes, err := json.Marshal(records)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d record set(s) in %s", len(records), zoneID),
		"records":     string(jsonBytes),
		"count":       len(records),
	}, nil
}
