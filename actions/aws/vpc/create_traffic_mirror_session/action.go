// Package aws_vpc_create_traffic_mirror_session creates a VPC Traffic Mirror session.
package aws_vpc_create_traffic_mirror_session

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
	Name         = "AWS VPC Create Traffic Mirror Session"
	Description  = "Create a VPC Traffic Mirror session from a source ENI to a target with a filter."
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
	{Name: "network_interface_id", Type: core.ConnectionTypeString, Label: "Source Network Interface ID", Placeholder: "eni-0abc", Required: true},
	{Name: "traffic_mirror_target_id", Type: core.ConnectionTypeString, Label: "Traffic Mirror Target ID", Placeholder: "tmt-0abc", Required: true},
	{Name: "traffic_mirror_filter_id", Type: core.ConnectionTypeString, Label: "Traffic Mirror Filter ID", Placeholder: "tmf-0abc", Required: true},
	{Name: "session_number", Type: core.ConnectionTypeInteger, Label: "Session Number", Placeholder: "1-32766 (evaluation order for the source ENI)", Required: true},
	{Name: "packet_length", Type: core.ConnectionTypeInteger, Label: "Packet Length (optional)", Placeholder: "Bytes to mirror after the VXLAN header"},
	{Name: "virtual_network_id", Type: core.ConnectionTypeInteger, Label: "Virtual Network ID (optional)", Placeholder: "VXLAN ID; random if left blank"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description (optional)"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "session", Type: core.ConnectionTypeObject, Label: "Traffic Mirror Session"},
	{Name: "traffic_mirror_session_id", Type: core.ConnectionTypeString, Label: "Traffic Mirror Session ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	eni := strings.TrimSpace(awscommon.InputString("network_interface_id", inputs))
	if eni == "" {
		return nil, fmt.Errorf("network_interface_id is required")
	}
	targetID := strings.TrimSpace(awscommon.InputString("traffic_mirror_target_id", inputs))
	if targetID == "" {
		return nil, fmt.Errorf("traffic_mirror_target_id is required")
	}
	filterID := strings.TrimSpace(awscommon.InputString("traffic_mirror_filter_id", inputs))
	if filterID == "" {
		return nil, fmt.Errorf("traffic_mirror_filter_id is required")
	}
	sessionNumber, ok := awscommon.InputInt("session_number", inputs)
	if !ok {
		return nil, fmt.Errorf("session_number is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.CreateTrafficMirrorSessionInput{
		NetworkInterfaceId:    aws.String(eni),
		TrafficMirrorTargetId: aws.String(targetID),
		TrafficMirrorFilterId: aws.String(filterID),
		SessionNumber:         aws.Int32(int32(sessionNumber)),
	}
	if p, ok := awscommon.InputInt("packet_length", inputs); ok {
		in.PacketLength = aws.Int32(int32(p))
	}
	if v, ok := awscommon.InputInt("virtual_network_id", inputs); ok {
		in.VirtualNetworkId = aws.Int32(int32(v))
	}
	if d := strings.TrimSpace(awscommon.InputString("description", inputs)); d != "" {
		in.Description = aws.String(d)
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.TagSpecifications = []ec2types.TagSpecification{{
			ResourceType: ec2types.ResourceTypeTrafficMirrorSession,
			Tags:         tags,
		}}
	}

	out, err := client.CreateTrafficMirrorSession(ctx, in)
	if err != nil {
		return nil, err
	}

	session := map[string]interface{}{}
	id := ""
	if out.TrafficMirrorSession != nil {
		s := out.TrafficMirrorSession
		id = aws.ToString(s.TrafficMirrorSessionId)
		session = map[string]interface{}{
			"traffic_mirror_session_id": id,
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
		"tool_result":               fmt.Sprintf("Created Traffic Mirror session %s", id),
		"session":                   session,
		"traffic_mirror_session_id": id,
	}, nil
}

func buildTags(inputs []*core.Connection) []ec2types.Tag {
	conn := core.FindConnection("tags", inputs)
	if conn == nil {
		return nil
	}
	var tags []ec2types.Tag
	for _, kv := range conn.KeyValuePairs() {
		k := strings.TrimSpace(kv.Key)
		if k == "" {
			continue
		}
		tags = append(tags, ec2types.Tag{Key: aws.String(k), Value: aws.String(kv.Value)})
	}
	return tags
}
