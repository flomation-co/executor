// Package aws_vpc_modify_traffic_mirror_session updates a VPC Traffic Mirror session.
package aws_vpc_modify_traffic_mirror_session

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
	Name         = "AWS VPC Modify Traffic Mirror Session"
	Description  = "Update a VPC Traffic Mirror session's number, packet length, or description."
	Website      = "https://www.flomation.co"
	Icon         = "copy+pen"
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
	{Name: "traffic_mirror_session_id", Type: core.ConnectionTypeString, Label: "Traffic Mirror Session ID", Placeholder: "tms-0abc", Required: true},
	{Name: "session_number", Type: core.ConnectionTypeInteger, Label: "Session Number (optional)", Placeholder: "1-32766"},
	{Name: "packet_length", Type: core.ConnectionTypeInteger, Label: "Packet Length (optional)", Placeholder: "Bytes to mirror after the VXLAN header"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "session", Type: core.ConnectionTypeObject, Label: "Traffic Mirror Session"},
	{Name: "traffic_mirror_session_id", Type: core.ConnectionTypeString, Label: "Traffic Mirror Session ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	id := strings.TrimSpace(awscommon.InputString("traffic_mirror_session_id", inputs))
	if id == "" {
		return nil, fmt.Errorf("traffic_mirror_session_id is required")
	}

	in := &ec2.ModifyTrafficMirrorSessionInput{TrafficMirrorSessionId: aws.String(id)}
	changed := false
	if n, ok := awscommon.InputInt("session_number", inputs); ok {
		in.SessionNumber = aws.Int32(int32(n))
		changed = true
	}
	if p, ok := awscommon.InputInt("packet_length", inputs); ok {
		in.PacketLength = aws.Int32(int32(p))
		changed = true
	}
	if d := strings.TrimSpace(awscommon.InputString("description", inputs)); d != "" {
		in.Description = aws.String(d)
		changed = true
	}
	if !changed {
		return nil, fmt.Errorf("provide at least one of session_number, packet_length, or description to modify")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	out, err := client.ModifyTrafficMirrorSession(ctx, in)
	if err != nil {
		return nil, err
	}

	session := map[string]interface{}{}
	if out.TrafficMirrorSession != nil {
		s := out.TrafficMirrorSession
		session = map[string]interface{}{
			"traffic_mirror_session_id": aws.ToString(s.TrafficMirrorSessionId),
			"traffic_mirror_target_id":  aws.ToString(s.TrafficMirrorTargetId),
			"traffic_mirror_filter_id":  aws.ToString(s.TrafficMirrorFilterId),
			"network_interface_id":      aws.ToString(s.NetworkInterfaceId),
			"session_number":            aws.ToInt32(s.SessionNumber),
			"packet_length":             aws.ToInt32(s.PacketLength),
			"virtual_network_id":        aws.ToInt32(s.VirtualNetworkId),
			"description":               aws.ToString(s.Description),
		}
	}

	return map[string]interface{}{
		"tool_result":               fmt.Sprintf("Modified Traffic Mirror session %s", id),
		"session":                   session,
		"traffic_mirror_session_id": id,
	}, nil
}
