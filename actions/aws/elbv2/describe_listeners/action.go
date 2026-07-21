// Package aws_elbv2_describe_listeners lists ELBv2 listeners.
package aws_elbv2_describe_listeners

import (
	"context"
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS ELBv2 Describe Listeners"
	Description  = "List listeners for a load balancer or by ARN, with protocol and port."
	Website      = "https://www.flomation.co"
	Icon         = "network-wired+magnifying-glass"
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
	{Name: "load_balancer_arn", Type: core.ConnectionTypeString, Label: "Load Balancer ARN", Placeholder: "Optional — list all listeners on this load balancer"},
	{Name: "listener_arns", Type: core.ConnectionTypeString, Label: "Listener ARNs", Placeholder: "Optional, comma-separated"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "listeners", Type: core.ConnectionTypeString, Label: "Listeners (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := elasticloadbalancingv2.NewFromConfig(cfg)

	in := &elasticloadbalancingv2.DescribeListenersInput{}
	if lb := awscommon.InputString("load_balancer_arn", inputs); lb != "" {
		in.LoadBalancerArn = aws.String(lb)
	}
	if arns := awscommon.InputStrings("listener_arns", inputs); len(arns) > 0 {
		in.ListenerArns = arns
	}

	type listenerInfo struct {
		ARN      string `json:"arn"`
		Protocol string `json:"protocol"`
		Port     int32  `json:"port"`
	}
	var listeners []listenerInfo

	paginator := elasticloadbalancingv2.NewDescribeListenersPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, l := range page.Listeners {
			listeners = append(listeners, listenerInfo{
				ARN:      aws.ToString(l.ListenerArn),
				Protocol: string(l.Protocol),
				Port:     aws.ToInt32(l.Port),
			})
		}
	}

	listenersJSON, err := json.Marshal(listeners)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d listener(s)", len(listeners)),
		"listeners":   string(listenersJSON),
		"count":       len(listeners),
	}, nil
}
