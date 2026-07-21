// Package aws_elbv2_create_listener creates an ELBv2 listener on a load balancer.
package aws_elbv2_create_listener

import (
	"context"
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS ELBv2 Create Listener"
	Description  = "Create a listener on an Elastic Load Balancing v2 load balancer."
	Website      = "https://www.flomation.co"
	Icon         = "network-wired+plus"
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
	{Name: "load_balancer_arn", Type: core.ConnectionTypeString, Label: "Load Balancer ARN", Placeholder: "arn:aws:elasticloadbalancing:...:loadbalancer/app/my-lb/...", Required: true},
	{Name: "protocol", Type: core.ConnectionTypeString, Label: "Protocol", Required: true, Options: []core.ConnectionOption{
		{Name: "HTTP", Value: "HTTP"},
		{Name: "HTTPS", Value: "HTTPS"},
		{Name: "TCP", Value: "TCP"},
		{Name: "TLS", Value: "TLS"},
		{Name: "UDP", Value: "UDP"},
		{Name: "TCP_UDP", Value: "TCP_UDP"},
	}},
	{Name: "port", Type: core.ConnectionTypeInteger, Label: "Port", Placeholder: "443", Required: true},
	{Name: "default_target_group_arn", Type: core.ConnectionTypeString, Label: "Default Target Group ARN", Placeholder: "Forwards to this target group (simple case)"},
	{Name: "certificate_arn", Type: core.ConnectionTypeString, Label: "Certificate ARN (HTTPS/TLS)", Placeholder: "Optional"},
	{Name: "ssl_policy", Type: core.ConnectionTypeString, Label: "SSL Policy (HTTPS/TLS)", Placeholder: "Optional, e.g. ELBSecurityPolicy-2016-08"},
	{Name: "default_actions", Type: core.ConnectionTypeString, Label: "Default Actions (JSON override)", Placeholder: "Optional JSON array of ELBv2 actions"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "listener_arn", Type: core.ConnectionTypeString, Label: "Listener ARN"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	lbARN := awscommon.InputString("load_balancer_arn", inputs)
	if lbARN == "" {
		return nil, fmt.Errorf("load balancer arn is required")
	}
	protocol := awscommon.InputString("protocol", inputs)
	if protocol == "" {
		return nil, fmt.Errorf("protocol is required")
	}
	port, ok := awscommon.InputInt("port", inputs)
	if !ok {
		return nil, fmt.Errorf("port is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := elasticloadbalancingv2.NewFromConfig(cfg)

	in := &elasticloadbalancingv2.CreateListenerInput{
		LoadBalancerArn: aws.String(lbARN),
		Protocol:        elbv2types.ProtocolEnum(protocol),
		Port:            aws.Int32(int32(port)),
	}

	if raw := awscommon.InputString("default_actions", inputs); raw != "" {
		var actions []elbv2types.Action
		if err := json.Unmarshal([]byte(raw), &actions); err != nil {
			return nil, fmt.Errorf("invalid default_actions JSON: %w", err)
		}
		in.DefaultActions = actions
	} else if tg := awscommon.InputString("default_target_group_arn", inputs); tg != "" {
		in.DefaultActions = []elbv2types.Action{{Type: elbv2types.ActionTypeEnumForward, TargetGroupArn: aws.String(tg)}}
	} else {
		return nil, fmt.Errorf("provide a default target group ARN or a default_actions JSON override")
	}

	if cert := awscommon.InputString("certificate_arn", inputs); cert != "" {
		in.Certificates = []elbv2types.Certificate{{CertificateArn: aws.String(cert)}}
	}
	if ssl := awscommon.InputString("ssl_policy", inputs); ssl != "" {
		in.SslPolicy = aws.String(ssl)
	}

	out, err := client.CreateListener(ctx, in)
	if err != nil {
		return nil, err
	}
	if len(out.Listeners) == 0 {
		return nil, fmt.Errorf("no listener returned")
	}

	listenerARN := aws.ToString(out.Listeners[0].ListenerArn)
	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Created %s listener on port %d: %s", protocol, port, listenerARN),
		"listener_arn": listenerARN,
	}, nil
}
