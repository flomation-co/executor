// Package aws_vpc_create_traffic_mirror_target creates a VPC Traffic Mirror target.
package aws_vpc_create_traffic_mirror_target

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
	Name         = "AWS VPC Create Traffic Mirror Target"
	Description  = "Create a VPC Traffic Mirror target (ENI, NLB, or Gateway LB endpoint)."
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
	{Name: "network_interface_id", Type: core.ConnectionTypeString, Label: "Network Interface ID (optional)", Placeholder: "eni-0abc"},
	{Name: "network_load_balancer_arn", Type: core.ConnectionTypeString, Label: "Network Load Balancer ARN (optional)", Placeholder: "arn:aws:elasticloadbalancing:..."},
	{Name: "gateway_load_balancer_endpoint_id", Type: core.ConnectionTypeString, Label: "Gateway Load Balancer Endpoint ID (optional)", Placeholder: "vpce-0abc"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description (optional)"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "target", Type: core.ConnectionTypeObject, Label: "Traffic Mirror Target"},
	{Name: "traffic_mirror_target_id", Type: core.ConnectionTypeString, Label: "Traffic Mirror Target ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	eni := strings.TrimSpace(awscommon.InputString("network_interface_id", inputs))
	nlb := strings.TrimSpace(awscommon.InputString("network_load_balancer_arn", inputs))
	glb := strings.TrimSpace(awscommon.InputString("gateway_load_balancer_endpoint_id", inputs))
	if eni == "" && nlb == "" && glb == "" {
		return nil, fmt.Errorf("provide one of network_interface_id, network_load_balancer_arn, or gateway_load_balancer_endpoint_id")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.CreateTrafficMirrorTargetInput{}
	if eni != "" {
		in.NetworkInterfaceId = aws.String(eni)
	}
	if nlb != "" {
		in.NetworkLoadBalancerArn = aws.String(nlb)
	}
	if glb != "" {
		in.GatewayLoadBalancerEndpointId = aws.String(glb)
	}
	if d := strings.TrimSpace(awscommon.InputString("description", inputs)); d != "" {
		in.Description = aws.String(d)
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.TagSpecifications = []ec2types.TagSpecification{{
			ResourceType: ec2types.ResourceTypeTrafficMirrorTarget,
			Tags:         tags,
		}}
	}

	out, err := client.CreateTrafficMirrorTarget(ctx, in)
	if err != nil {
		return nil, err
	}

	target := map[string]interface{}{}
	id := ""
	if out.TrafficMirrorTarget != nil {
		t := out.TrafficMirrorTarget
		id = aws.ToString(t.TrafficMirrorTargetId)
		target = map[string]interface{}{
			"traffic_mirror_target_id":          id,
			"type":                              string(t.Type),
			"network_interface_id":              aws.ToString(t.NetworkInterfaceId),
			"network_load_balancer_arn":         aws.ToString(t.NetworkLoadBalancerArn),
			"gateway_load_balancer_endpoint_id": aws.ToString(t.GatewayLoadBalancerEndpointId),
			"description":                       aws.ToString(t.Description),
		}
	}

	return map[string]interface{}{
		"tool_result":              fmt.Sprintf("Created Traffic Mirror target %s", id),
		"target":                   target,
		"traffic_mirror_target_id": id,
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
