// Package aws_vpc_describe_traffic_mirror_sessions lists VPC Traffic Mirror sessions.
package aws_vpc_describe_traffic_mirror_sessions

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS VPC Describe Traffic Mirror Sessions"
	Description  = "List VPC Traffic Mirror sessions, optionally filtered by session id or tags."
	Website      = "https://www.flomation.co"
	Icon         = "copy+magnifying-glass"
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
	{Name: "traffic_mirror_session_id", Type: core.ConnectionTypeString, Label: "Traffic Mirror Session ID (optional)", Placeholder: "Leave blank to list all"},
	{Name: "filter_tags", Type: core.ConnectionTypeKeyValueArray, Label: "Filter by Tags (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "sessions", Type: core.ConnectionTypeObject, Label: "Traffic Mirror Sessions"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.DescribeTrafficMirrorSessionsInput{
		Filters: awscommon.BuildEC2Filters(inputs, []awscommon.FilterSpec{
			{Input: "traffic_mirror_session_id", Filter: "traffic-mirror-session-id"},
		}),
	}

	var sessions []map[string]interface{}
	for {
		page, err := client.DescribeTrafficMirrorSessions(ctx, in)
		if err != nil {
			return nil, err
		}
		for i := range page.TrafficMirrorSessions {
			s := &page.TrafficMirrorSessions[i]
			sessions = append(sessions, map[string]interface{}{
				"traffic_mirror_session_id": aws.ToString(s.TrafficMirrorSessionId),
				"traffic_mirror_target_id":  aws.ToString(s.TrafficMirrorTargetId),
				"traffic_mirror_filter_id":  aws.ToString(s.TrafficMirrorFilterId),
				"network_interface_id":      aws.ToString(s.NetworkInterfaceId),
				"session_number":            aws.ToInt32(s.SessionNumber),
				"packet_length":             aws.ToInt32(s.PacketLength),
				"virtual_network_id":        aws.ToInt32(s.VirtualNetworkId),
				"owner_id":                  aws.ToString(s.OwnerId),
				"description":               aws.ToString(s.Description),
				"name":                      tagName(s.Tags),
			})
		}
		if page.NextToken == nil || aws.ToString(page.NextToken) == "" {
			break
		}
		in.NextToken = page.NextToken
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d Traffic Mirror session(s)", len(sessions)),
		"sessions":    sessions,
		"count":       len(sessions),
	}, nil
}

func tagName(tags []ec2types.Tag) string {
	for _, t := range tags {
		if aws.ToString(t.Key) == "Name" {
			return aws.ToString(t.Value)
		}
	}
	return ""
}
