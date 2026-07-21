// Package aws_vpc_replace_network_acl_entry replaces an existing rule in a network ACL.
package aws_vpc_replace_network_acl_entry

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
	Name         = "AWS VPC Replace Network ACL Entry"
	Description  = "Replace an existing inbound or outbound rule in a network ACL."
	Website      = "https://www.flomation.co"
	Icon         = "lock+pen"
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
	{Name: "network_acl_id", Type: core.ConnectionTypeString, Label: "Network ACL ID", Placeholder: "acl-0abc", Required: true},
	{Name: "rule_number", Type: core.ConnectionTypeInteger, Label: "Rule Number", Placeholder: "100 (1-32766, processed ascending)", Required: true},
	{Name: "protocol", Type: core.ConnectionTypeString, Label: "Protocol Number", Placeholder: "6 = TCP, 17 = UDP, -1 = all", Required: true},
	{Name: "rule_action", Type: core.ConnectionTypeString, Label: "Rule Action", Required: true, Options: []core.ConnectionOption{
		{Name: "Allow", Value: "allow"},
		{Name: "Deny", Value: "deny"},
	}},
	{Name: "egress", Type: core.ConnectionTypeBoolean, Label: "Egress (outbound rule)", Required: true},
	{Name: "cidr_block", Type: core.ConnectionTypeString, Label: "CIDR Block", Placeholder: "0.0.0.0/0", Required: true},
	{Name: "port_from", Type: core.ConnectionTypeInteger, Label: "Port From (optional)", Placeholder: "For TCP/UDP, e.g. 80"},
	{Name: "port_to", Type: core.ConnectionTypeInteger, Label: "Port To (optional)", Placeholder: "For TCP/UDP, e.g. 80"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	aclID := strings.TrimSpace(awscommon.InputString("network_acl_id", inputs))
	if aclID == "" {
		return nil, fmt.Errorf("network_acl_id is required")
	}
	ruleNumber, ok := intInput("rule_number", inputs)
	if !ok {
		return nil, fmt.Errorf("rule_number is required")
	}
	protocol := strings.TrimSpace(awscommon.InputString("protocol", inputs))
	if protocol == "" {
		return nil, fmt.Errorf("protocol is required")
	}
	action := strings.TrimSpace(awscommon.InputString("rule_action", inputs))
	if action == "" {
		return nil, fmt.Errorf("rule_action is required")
	}
	cidr := strings.TrimSpace(awscommon.InputString("cidr_block", inputs))
	if cidr == "" {
		return nil, fmt.Errorf("cidr_block is required")
	}
	egress := false
	if c := core.FindConnection("egress", inputs); c != nil {
		if b := c.Boolean(); b != nil {
			egress = *b
		}
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.ReplaceNetworkAclEntryInput{
		NetworkAclId: aws.String(aclID),
		RuleNumber:   aws.Int32(ruleNumber),
		Protocol:     aws.String(protocol),
		RuleAction:   ec2types.RuleAction(action),
		Egress:       aws.Bool(egress),
		CidrBlock:    aws.String(cidr),
	}
	if from, ok := intInput("port_from", inputs); ok {
		pr := &ec2types.PortRange{From: aws.Int32(from)}
		if to, ok := intInput("port_to", inputs); ok {
			pr.To = aws.Int32(to)
		} else {
			pr.To = aws.Int32(from)
		}
		in.PortRange = pr
	} else if to, ok := intInput("port_to", inputs); ok {
		in.PortRange = &ec2types.PortRange{From: aws.Int32(to), To: aws.Int32(to)}
	}

	if _, err := client.ReplaceNetworkAclEntry(ctx, in); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Replaced rule %d (%s) in network ACL %s", ruleNumber, action, aclID),
	}, nil
}

func intInput(name string, inputs []*core.Connection) (int32, bool) {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return 0, false
	}
	n := c.Number()
	if n == nil {
		return 0, false
	}
	return int32(*n), true
}
