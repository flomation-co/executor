// Package aws_vpc_create_traffic_mirror_filter_rule adds a rule to a VPC Traffic Mirror filter.
package aws_vpc_create_traffic_mirror_filter_rule

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS VPC Create Traffic Mirror Filter Rule"
	Description  = "Add an ingress or egress rule to a VPC Traffic Mirror filter."
	Website      = "https://www.flomation.co"
	Icon         = "copy+plus"
	Date         = "21/07/2026"
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
	{Name: "traffic_mirror_filter_id", Type: core.ConnectionTypeString, Label: "Traffic Mirror Filter ID", Placeholder: "tmf-0abc", Required: true},
	{Name: "traffic_direction", Type: core.ConnectionTypeString, Label: "Traffic Direction", Required: true, Options: []core.ConnectionOption{
		{Name: "Ingress", Value: "ingress"},
		{Name: "Egress", Value: "egress"},
	}},
	{Name: "rule_number", Type: core.ConnectionTypeInteger, Label: "Rule Number", Placeholder: "e.g. 100 (unique per direction, evaluated ascending)", Required: true},
	{Name: "rule_action", Type: core.ConnectionTypeString, Label: "Rule Action", Required: true, Options: []core.ConnectionOption{
		{Name: "Accept", Value: "accept"},
		{Name: "Reject", Value: "reject"},
	}},
	{Name: "protocol", Type: core.ConnectionTypeInteger, Label: "Protocol Number (optional)", Placeholder: "e.g. 6 (TCP), 17 (UDP)"},
	{Name: "destination_cidr_block", Type: core.ConnectionTypeString, Label: "Destination CIDR Block", Placeholder: "0.0.0.0/0", Required: true},
	{Name: "source_cidr_block", Type: core.ConnectionTypeString, Label: "Source CIDR Block", Placeholder: "10.0.0.0/16", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "rule", Type: core.ConnectionTypeObject, Label: "Traffic Mirror Filter Rule"},
	{Name: "traffic_mirror_filter_rule_id", Type: core.ConnectionTypeString, Label: "Traffic Mirror Filter Rule ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	filterID := strings.TrimSpace(awscommon.InputString("traffic_mirror_filter_id", inputs))
	if filterID == "" {
		return nil, fmt.Errorf("traffic_mirror_filter_id is required")
	}
	direction := strings.TrimSpace(awscommon.InputString("traffic_direction", inputs))
	if direction == "" {
		return nil, fmt.Errorf("traffic_direction is required")
	}
	action := strings.TrimSpace(awscommon.InputString("rule_action", inputs))
	if action == "" {
		return nil, fmt.Errorf("rule_action is required")
	}
	ruleNumber, ok := awscommon.InputInt("rule_number", inputs)
	if !ok {
		return nil, fmt.Errorf("rule_number is required")
	}
	destCIDR := strings.TrimSpace(awscommon.InputString("destination_cidr_block", inputs))
	if destCIDR == "" {
		return nil, fmt.Errorf("destination_cidr_block is required")
	}
	srcCIDR := strings.TrimSpace(awscommon.InputString("source_cidr_block", inputs))
	if srcCIDR == "" {
		return nil, fmt.Errorf("source_cidr_block is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.CreateTrafficMirrorFilterRuleInput{
		TrafficMirrorFilterId: aws.String(filterID),
		TrafficDirection:      ec2types.TrafficDirection(direction),
		RuleAction:            ec2types.TrafficMirrorRuleAction(action),
		RuleNumber:            aws.Int32(int32(ruleNumber)),
		DestinationCidrBlock:  aws.String(destCIDR),
		SourceCidrBlock:       aws.String(srcCIDR),
	}
	if p, ok := awscommon.InputInt("protocol", inputs); ok {
		in.Protocol = aws.Int32(int32(p))
	}
	if d := strings.TrimSpace(awscommon.InputString("description", inputs)); d != "" {
		in.Description = aws.String(d)
	}

	out, err := client.CreateTrafficMirrorFilterRule(ctx, in)
	if err != nil {
		return nil, err
	}

	rule := map[string]interface{}{}
	id := ""
	if out.TrafficMirrorFilterRule != nil {
		r := out.TrafficMirrorFilterRule
		id = aws.ToString(r.TrafficMirrorFilterRuleId)
		rule = map[string]interface{}{
			"traffic_mirror_filter_rule_id": id,
			"traffic_mirror_filter_id":      aws.ToString(r.TrafficMirrorFilterId),
			"traffic_direction":             string(r.TrafficDirection),
			"rule_number":                   aws.ToInt32(r.RuleNumber),
			"rule_action":                   string(r.RuleAction),
			"protocol":                      aws.ToInt32(r.Protocol),
			"destination_cidr_block":        aws.ToString(r.DestinationCidrBlock),
			"source_cidr_block":             aws.ToString(r.SourceCidrBlock),
			"description":                   aws.ToString(r.Description),
		}
	}

	return map[string]interface{}{
		"tool_result":                   fmt.Sprintf("Created Traffic Mirror filter rule %s on %s", id, filterID),
		"rule":                          rule,
		"traffic_mirror_filter_rule_id": id,
	}, nil
}
