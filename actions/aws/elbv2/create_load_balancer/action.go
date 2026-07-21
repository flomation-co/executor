// Package aws_elbv2_create_load_balancer creates an Elastic Load Balancing v2
// load balancer (Application, Network or Gateway).
package aws_elbv2_create_load_balancer

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Create Load Balancer"
	Description  = "Create an Elastic Load Balancing v2 load balancer (application, network or gateway)."
	Website      = "https://www.flomation.co"
	Icon         = "arrows-split-up-and-left+plus"
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
	{Name: "name", Type: core.ConnectionTypeString, Label: "Load Balancer Name", Placeholder: "my-load-balancer", Required: true},
	{Name: "subnets", Type: core.ConnectionTypeString, Label: "Subnet IDs", Placeholder: "Comma-separated subnet IDs (required for ALB)"},
	{Name: "security_groups", Type: core.ConnectionTypeString, Label: "Security Group IDs", Placeholder: "Comma-separated (optional)"},
	{Name: "scheme", Type: core.ConnectionTypeString, Label: "Scheme", Options: []core.ConnectionOption{
		{Name: "Internet-facing", Value: "internet-facing"},
		{Name: "Internal", Value: "internal"},
	}},
	{Name: "type", Type: core.ConnectionTypeString, Label: "Type", Options: []core.ConnectionOption{
		{Name: "Application", Value: "application"},
		{Name: "Network", Value: "network"},
		{Name: "Gateway", Value: "gateway"},
	}},
	{Name: "ip_address_type", Type: core.ConnectionTypeString, Label: "IP Address Type (optional)", Options: []core.ConnectionOption{
		{Name: "IPv4", Value: "ipv4"},
		{Name: "Dualstack", Value: "dualstack"},
	}},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags", Placeholder: "Optional tags for the load balancer"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "load_balancer_arn", Type: core.ConnectionTypeString, Label: "Load Balancer ARN"},
	{Name: "dns_name", Type: core.ConnectionTypeString, Label: "DNS Name"},
	{Name: "load_balancer_name", Type: core.ConnectionTypeString, Label: "Load Balancer Name"},
	{Name: "state", Type: core.ConnectionTypeString, Label: "State"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	name := strings.TrimSpace(awscommon.InputString("name", inputs))
	if name == "" {
		return nil, fmt.Errorf("load balancer name is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := elbv2.NewFromConfig(cfg)

	in := &elbv2.CreateLoadBalancerInput{Name: aws.String(name)}
	if subnets := awscommon.InputStrings("subnets", inputs); len(subnets) > 0 {
		in.Subnets = subnets
	}
	if sgs := awscommon.InputStrings("security_groups", inputs); len(sgs) > 0 {
		in.SecurityGroups = sgs
	}
	if scheme := strings.TrimSpace(awscommon.InputString("scheme", inputs)); scheme != "" {
		in.Scheme = elbv2types.LoadBalancerSchemeEnum(scheme)
	}
	if lbType := strings.TrimSpace(awscommon.InputString("type", inputs)); lbType != "" {
		in.Type = elbv2types.LoadBalancerTypeEnum(lbType)
	}
	if ipType := strings.TrimSpace(awscommon.InputString("ip_address_type", inputs)); ipType != "" {
		in.IpAddressType = elbv2types.IpAddressType(ipType)
	}
	if tags := tagsFromInput(inputs); len(tags) > 0 {
		in.Tags = tags
	}

	out, err := client.CreateLoadBalancer(ctx, in)
	if err != nil {
		return nil, err
	}
	if len(out.LoadBalancers) == 0 {
		return nil, fmt.Errorf("AWS returned no load balancer")
	}

	lb := out.LoadBalancers[0]
	arn := aws.ToString(lb.LoadBalancerArn)
	dnsName := aws.ToString(lb.DNSName)
	lbName := aws.ToString(lb.LoadBalancerName)
	var state string
	if lb.State != nil {
		state = string(lb.State.Code)
	}

	return map[string]interface{}{
		"tool_result":        fmt.Sprintf("Created load balancer %s (%s), state: %s", lbName, arn, state),
		"load_balancer_arn":  arn,
		"dns_name":           dnsName,
		"load_balancer_name": lbName,
		"state":              state,
	}, nil
}

func tagsFromInput(inputs []*core.Connection) []elbv2types.Tag {
	conn := core.FindConnection("tags", inputs)
	if conn == nil {
		return nil
	}
	var tags []elbv2types.Tag
	for _, kv := range conn.KeyValuePairs() {
		k := strings.TrimSpace(kv.Key)
		if k == "" {
			continue
		}
		tags = append(tags, elbv2types.Tag{Key: aws.String(k), Value: aws.String(strings.TrimSpace(kv.Value))})
	}
	return tags
}
