// Package aws_ec2_create_security_group creates a new EC2 security group.
package aws_ec2_create_security_group

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS EC2 Create Security Group"
	Description  = "Create a security group in a VPC. Rules are added separately via authorize ingress."
	Website      = "https://www.flomation.co"
	Icon         = "shield-halved+plus"
	Date         = "17/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "aws_access_key", Type: core.ConnectionTypeSecret, Label: "AWS Access Key", Required: true},
	{Name: "aws_secret_key", Type: core.ConnectionTypeSecret, Label: "AWS Secret Key", Required: true},
	{Name: "aws_region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "eu-west-2", Required: true},
	{Name: "aws_session_token", Type: core.ConnectionTypeSecret, Label: "Session Token (optional)"},
	{Name: "assume_role_arn", Type: core.ConnectionTypeString, Label: "Assume Role ARN (optional)", Placeholder: "arn:aws:iam::123456789012:role/MyRole"},
	{Name: "group_name", Type: core.ConnectionTypeString, Label: "Group Name", Placeholder: "web-sg", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "Allows web traffic", Required: true},
	{Name: "vpc_id", Type: core.ConnectionTypeString, Label: "VPC ID", Placeholder: "vpc-0abc (optional; default VPC if blank)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "group_id", Type: core.ConnectionTypeString, Label: "Group ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(awscommon.InputString("group_name", inputs)),
		Description: aws.String(awscommon.InputString("description", inputs)),
	}
	if v := awscommon.InputString("vpc_id", inputs); v != "" {
		in.VpcId = aws.String(v)
	}

	out, err := client.CreateSecurityGroup(ctx, in)
	if err != nil {
		return nil, err
	}

	groupID := aws.ToString(out.GroupId)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created security group %s", groupID),
		"group_id":    groupID,
	}, nil
}
