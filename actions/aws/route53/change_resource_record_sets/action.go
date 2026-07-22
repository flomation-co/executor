// Package aws_route53_change_resource_record_sets creates, deletes or upserts
// DNS records in a Route 53 hosted zone.
package aws_route53_change_resource_record_sets

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
	Name         = "AWS Route 53 Change Records"
	Description  = "Create/delete/upsert DNS records; record_set JSON enables alias/weighted routing."
	Website      = "https://www.flomation.co"
	Icon         = "route+pen"
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
	{Name: "action", Type: core.ConnectionTypeString, Label: "Action", Required: true, Options: []core.ConnectionOption{
		{Name: "Create", Value: "CREATE"},
		{Name: "Delete", Value: "DELETE"},
		{Name: "Upsert (create or update)", Value: "UPSERT"},
	}},
	{Name: "record_name", Type: core.ConnectionTypeString, Label: "Record Name", Placeholder: "www.example.com", Required: true},
	{Name: "record_type", Type: core.ConnectionTypeString, Label: "Record Type", Options: []core.ConnectionOption{
		{Name: "A", Value: "A"},
		{Name: "AAAA", Value: "AAAA"},
		{Name: "CNAME", Value: "CNAME"},
		{Name: "MX", Value: "MX"},
		{Name: "TXT", Value: "TXT"},
		{Name: "NS", Value: "NS"},
		{Name: "SRV", Value: "SRV"},
		{Name: "CAA", Value: "CAA"},
		{Name: "PTR", Value: "PTR"},
		{Name: "SOA", Value: "SOA"},
	}},
	{Name: "ttl", Type: core.ConnectionTypeInteger, Label: "TTL (seconds)", Placeholder: "300"},
	{Name: "values", Type: core.ConnectionTypeString, Label: "Values (comma-separated)", Placeholder: "192.0.2.1, 192.0.2.2"},
	{Name: "comment", Type: core.ConnectionTypeString, Label: "Comment (optional)"},
	{Name: "record_set", Type: core.ConnectionTypeString, Label: "Record Set JSON (override for alias/weighted/latency/failover routing)", Placeholder: `{"Name":"www.example.com","Type":"A","AliasTarget":{...}}`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "change_id", Type: core.ConnectionTypeString, Label: "Change ID"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	zoneID := awscommon.InputString("hosted_zone_id", inputs)
	if zoneID == "" {
		return nil, fmt.Errorf("hosted zone id is required")
	}
	action := strings.TrimSpace(awscommon.InputString("action", inputs))
	if action == "" {
		return nil, fmt.Errorf("action (CREATE/DELETE/UPSERT) is required")
	}

	var rrs r53types.ResourceRecordSet
	if override := strings.TrimSpace(awscommon.InputString("record_set", inputs)); override != "" {
		// Full JSON override for advanced routing policies (alias/weighted/etc).
		if err := json.Unmarshal([]byte(override), &rrs); err != nil {
			return nil, fmt.Errorf("record_set must be a JSON ResourceRecordSet: %w", err)
		}
	} else {
		recordName := awscommon.InputString("record_name", inputs)
		if recordName == "" {
			return nil, fmt.Errorf("record_name is required (or provide record_set JSON)")
		}
		recordType := strings.TrimSpace(awscommon.InputString("record_type", inputs))
		if recordType == "" {
			return nil, fmt.Errorf("record_type is required (or provide record_set JSON)")
		}
		ttl := int64(300)
		if v, ok := awscommon.InputInt("ttl", inputs); ok {
			ttl = v
		}
		var records []r53types.ResourceRecord
		for _, v := range splitCSV(awscommon.InputString("values", inputs)) {
			records = append(records, r53types.ResourceRecord{Value: aws.String(v)})
		}
		if len(records) == 0 {
			return nil, fmt.Errorf("at least one value is required (or provide record_set JSON)")
		}
		rrs = r53types.ResourceRecordSet{
			Name:            aws.String(recordName),
			Type:            r53types.RRType(recordType),
			TTL:             aws.Int64(ttl),
			ResourceRecords: records,
		}
	}

	batch := &r53types.ChangeBatch{
		Changes: []r53types.Change{{
			Action:            r53types.ChangeAction(action),
			ResourceRecordSet: &rrs,
		}},
	}
	if comment := strings.TrimSpace(awscommon.InputString("comment", inputs)); comment != "" {
		batch.Comment = aws.String(comment)
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := route53.NewFromConfig(cfg)

	out, err := client.ChangeResourceRecordSets(ctx, &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(zoneID),
		ChangeBatch:  batch,
	})
	if err != nil {
		return nil, err
	}

	changeID := aws.ToString(out.ChangeInfo.Id)
	status := string(out.ChangeInfo.Status)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("%s change submitted (change %s, status %s)", action, changeID, status),
		"change_id":   changeID,
		"status":      status,
	}, nil
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
