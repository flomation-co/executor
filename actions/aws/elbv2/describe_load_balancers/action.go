// Package aws_elbv2_describe_load_balancers lists Elastic Load Balancing v2 load
// balancers and their key details.
package aws_elbv2_describe_load_balancers

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Describe Load Balancers"
	Description  = "List Elastic Load Balancing v2 load balancers with their scheme, type, state and VPC."
	Website      = "https://www.flomation.co"
	Icon         = "arrows-split-up-and-left+magnifying-glass"
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
	{Name: "load_balancer_arns", Type: core.ConnectionTypeString, Label: "Load Balancer ARNs", Placeholder: "Comma-separated; leave blank for all (optional)"},
	{Name: "names", Type: core.ConnectionTypeString, Label: "Load Balancer Names", Placeholder: "Comma-separated; leave blank for all (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "load_balancers", Type: core.ConnectionTypeObject, Label: "Load Balancers"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := elbv2.NewFromConfig(cfg)

	in := &elbv2.DescribeLoadBalancersInput{}
	if arns := awscommon.InputStrings("load_balancer_arns", inputs); len(arns) > 0 {
		in.LoadBalancerArns = arns
	}
	if names := awscommon.InputStrings("names", inputs); len(names) > 0 {
		in.Names = names
	}

	var loadBalancers []map[string]interface{}
	paginator := elbv2.NewDescribeLoadBalancersPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, lb := range page.LoadBalancers {
			loadBalancers = append(loadBalancers, summariseLoadBalancer(lb))
		}
	}

	return map[string]interface{}{
		"tool_result":    fmt.Sprintf("Found %d load balancer(s)", len(loadBalancers)),
		"load_balancers": loadBalancers,
		"count":          len(loadBalancers),
	}, nil
}

func summariseLoadBalancer(lb elbv2types.LoadBalancer) map[string]interface{} {
	m := map[string]interface{}{
		"arn":      aws.ToString(lb.LoadBalancerArn),
		"name":     aws.ToString(lb.LoadBalancerName),
		"dns_name": aws.ToString(lb.DNSName),
		"scheme":   string(lb.Scheme),
		"type":     string(lb.Type),
		"vpc_id":   aws.ToString(lb.VpcId),
	}
	if lb.State != nil {
		m["state"] = string(lb.State.Code)
	} else {
		m["state"] = ""
	}
	return m
}
