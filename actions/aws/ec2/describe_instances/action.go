// Package aws_ec2_describe_instances lists EC2 instances and their key details.
// It is the reference implementation for the AWS action template: an inline
// credential input block (aws_access_key/secret/region + optional session token
// and assume-role ARN), tool_result as the first output, and Execute delegating
// credential loading to the shared awscommon.ConfigFromInputs helper.
package aws_ec2_describe_instances

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS EC2 Describe Instances"
	Description  = "List EC2 instances with their state, type, IPs, AZ and tags."
	Website      = "https://www.flomation.co"
	Icon         = "server"
	Date         = "17/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "aws_access_key", Type: core.ConnectionTypeSecret, Label: "AWS Access Key", Required: true},
	{Name: "aws_secret_key", Type: core.ConnectionTypeSecret, Label: "AWS Secret Key", Required: true},
	{Name: "aws_region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "eu-west-2", Required: true},
	{Name: "aws_session_token", Type: core.ConnectionTypeSecret, Label: "Session Token (optional)"},
	{Name: "assume_role_arn", Type: core.ConnectionTypeString, Label: "Assume Role ARN (optional)", Placeholder: "arn:aws:iam::123456789012:role/MyRole"},
	{Name: "external_id", Type: core.ConnectionTypeString, Label: "Assume Role External ID (optional)", Placeholder: "Must match the External ID in the role's trust policy"},
	{Name: "instance_ids", Type: core.ConnectionTypeString, Label: "Instance IDs", Placeholder: "Comma-separated; leave blank for all (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "instances", Type: core.ConnectionTypeObject, Label: "Instances"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.DescribeInstancesInput{}
	if ids := awscommon.InputStrings("instance_ids", inputs); len(ids) > 0 {
		in.InstanceIds = ids
	}

	var instances []map[string]interface{}
	paginator := ec2.NewDescribeInstancesPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, res := range page.Reservations {
			for _, inst := range res.Instances {
				instances = append(instances, summariseInstance(inst))
			}
		}
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d EC2 instance(s)", len(instances)),
		"instances":   instances,
		"count":       len(instances),
	}, nil
}

// summariseInstance flattens the SDK instance into a compact, JSON-friendly map.
func summariseInstance(inst types.Instance) map[string]interface{} {
	m := map[string]interface{}{
		"instance_id":   aws.ToString(inst.InstanceId),
		"instance_type": string(inst.InstanceType),
		"private_ip":    aws.ToString(inst.PrivateIpAddress),
		"public_ip":     aws.ToString(inst.PublicIpAddress),
		"subnet_id":     aws.ToString(inst.SubnetId),
		"vpc_id":        aws.ToString(inst.VpcId),
		"image_id":      aws.ToString(inst.ImageId),
	}
	if inst.State != nil {
		m["state"] = string(inst.State.Name)
	}
	if inst.Placement != nil {
		m["availability_zone"] = aws.ToString(inst.Placement.AvailabilityZone)
	}
	if inst.LaunchTime != nil {
		m["launch_time"] = inst.LaunchTime.UTC().Format("2006-01-02T15:04:05Z")
	}
	tags := map[string]string{}
	for _, t := range inst.Tags {
		tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
	}
	m["tags"] = tags
	if name, ok := tags["Name"]; ok {
		m["name"] = name
	}
	return m
}
