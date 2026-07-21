// Package aws_vpc_create_vpc_endpoint creates a VPC endpoint to an AWS service.
package aws_vpc_create_vpc_endpoint

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
	Name         = "AWS VPC Create VPC Endpoint"
	Description  = "Create a Gateway or Interface VPC endpoint to an AWS service."
	Website      = "https://www.flomation.co"
	Icon         = "link+plus"
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
	{Name: "vpc_id", Type: core.ConnectionTypeString, Label: "VPC ID", Placeholder: "vpc-0abc", Required: true},
	{Name: "service_name", Type: core.ConnectionTypeString, Label: "Service Name", Placeholder: "com.amazonaws.eu-west-2.s3", Required: true},
	{Name: "vpc_endpoint_type", Type: core.ConnectionTypeString, Label: "Endpoint Type", Options: []core.ConnectionOption{
		{Name: "Gateway", Value: "Gateway"},
		{Name: "Interface", Value: "Interface"},
		{Name: "Gateway Load Balancer", Value: "GatewayLoadBalancer"},
	}},
	{Name: "route_table_ids", Type: core.ConnectionTypeString, Label: "Route Table IDs (Gateway)", Placeholder: "Comma-separated, e.g. rtb-0abc,rtb-0def"},
	{Name: "subnet_ids", Type: core.ConnectionTypeString, Label: "Subnet IDs (Interface)", Placeholder: "Comma-separated, e.g. subnet-0abc"},
	{Name: "security_group_ids", Type: core.ConnectionTypeString, Label: "Security Group IDs (Interface)", Placeholder: "Comma-separated, e.g. sg-0abc"},
	{Name: "private_dns_enabled", Type: core.ConnectionTypeBoolean, Label: "Enable Private DNS (Interface)"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "vpc_endpoint", Type: core.ConnectionTypeObject, Label: "VPC Endpoint"},
	{Name: "vpc_endpoint_id", Type: core.ConnectionTypeString, Label: "VPC Endpoint ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	vpcID := strings.TrimSpace(awscommon.InputString("vpc_id", inputs))
	if vpcID == "" {
		return nil, fmt.Errorf("vpc_id is required")
	}
	serviceName := strings.TrimSpace(awscommon.InputString("service_name", inputs))
	if serviceName == "" {
		return nil, fmt.Errorf("service_name is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.CreateVpcEndpointInput{
		VpcId:       aws.String(vpcID),
		ServiceName: aws.String(serviceName),
	}
	if t := strings.TrimSpace(awscommon.InputString("vpc_endpoint_type", inputs)); t != "" {
		in.VpcEndpointType = ec2types.VpcEndpointType(t)
	}
	if v := awscommon.InputStrings("route_table_ids", inputs); len(v) > 0 {
		in.RouteTableIds = v
	}
	if v := awscommon.InputStrings("subnet_ids", inputs); len(v) > 0 {
		in.SubnetIds = v
	}
	if v := awscommon.InputStrings("security_group_ids", inputs); len(v) > 0 {
		in.SecurityGroupIds = v
	}
	if c := core.FindConnection("private_dns_enabled", inputs); c != nil {
		if b := c.Boolean(); b != nil {
			in.PrivateDnsEnabled = aws.Bool(*b)
		}
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.TagSpecifications = []ec2types.TagSpecification{{
			ResourceType: ec2types.ResourceTypeVpcEndpoint,
			Tags:         tags,
		}}
	}

	out, err := client.CreateVpcEndpoint(ctx, in)
	if err != nil {
		return nil, err
	}

	ep := map[string]interface{}{}
	id := ""
	if out.VpcEndpoint != nil {
		id = aws.ToString(out.VpcEndpoint.VpcEndpointId)
		ep = map[string]interface{}{
			"vpc_endpoint_id":   id,
			"vpc_id":            aws.ToString(out.VpcEndpoint.VpcId),
			"service_name":      aws.ToString(out.VpcEndpoint.ServiceName),
			"vpc_endpoint_type": string(out.VpcEndpoint.VpcEndpointType),
			"state":             string(out.VpcEndpoint.State),
		}
	}

	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Created VPC endpoint %s to %s", id, serviceName),
		"vpc_endpoint":    ep,
		"vpc_endpoint_id": id,
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
