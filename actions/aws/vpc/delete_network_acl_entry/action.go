// Package aws_vpc_delete_network_acl_entry removes a rule from a network ACL.
package aws_vpc_delete_network_acl_entry

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS VPC Delete Network ACL Entry"
	Description  = "Remove an inbound or outbound rule from a network ACL."
	Website      = "https://www.flomation.co"
	Icon         = "lock+trash"
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
	{Name: "rule_number", Type: core.ConnectionTypeInteger, Label: "Rule Number", Placeholder: "100", Required: true},
	{Name: "egress", Type: core.ConnectionTypeBoolean, Label: "Egress (outbound rule)", Required: true},
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
	c := core.FindConnection("rule_number", inputs)
	if c == nil || c.Number() == nil {
		return nil, fmt.Errorf("rule_number is required")
	}
	ruleNumber := int32(*c.Number())
	egress := false
	if e := core.FindConnection("egress", inputs); e != nil {
		if b := e.Boolean(); b != nil {
			egress = *b
		}
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.DeleteNetworkAclEntryInput{
		NetworkAclId: aws.String(aclID),
		RuleNumber:   aws.Int32(ruleNumber),
		Egress:       aws.Bool(egress),
	}
	if _, err := client.DeleteNetworkAclEntry(ctx, in); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Deleted rule %d from network ACL %s", ruleNumber, aclID),
	}, nil
}
