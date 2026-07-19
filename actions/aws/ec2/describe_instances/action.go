// Package aws_ec2_describe_instances lists EC2 instances and their key details.
// It is the reference implementation for the AWS action template: an inline
// credential input block (aws_access_key/secret/region + optional session token
// and assume-role ARN), tool_result as the first output, and Execute delegating
// credential loading to the shared awscommon.ConfigFromInputs helper.
package aws_ec2_describe_instances

import (
	"context"
	"fmt"
	"strings"

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
	{Name: "instance_ids", Type: core.ConnectionTypeString, Label: "Instance IDs", Placeholder: "Comma-separated; leave blank for all (optional)"},
	{Name: "filter_tags", Type: core.ConnectionTypeKeyValueArray, Label: "Filter by Tags", Placeholder: "Only return instances with these tags (blank Value matches any value for that key)"},
	{Name: "filter_state", Type: core.ConnectionTypeMultiSelect, Label: "Filter by State", Placeholder: "Select one or more; none = any state", Options: []core.ConnectionOption{
		{Name: "Running", Value: "running"},
		{Name: "Stopped", Value: "stopped"},
		{Name: "Pending", Value: "pending"},
		{Name: "Stopping", Value: "stopping"},
		{Name: "Shutting down", Value: "shutting-down"},
		{Name: "Terminated", Value: "terminated"},
	}},
	{Name: "filter_instance_type", Type: core.ConnectionTypeString, Label: "Filter by Instance Type", Placeholder: "e.g. t3.micro (optional)"},
	{Name: "filter_vpc_id", Type: core.ConnectionTypeString, Label: "Filter by VPC ID", Placeholder: "vpc-0abc (optional)"},
	{Name: "filter_subnet_id", Type: core.ConnectionTypeString, Label: "Filter by Subnet ID", Placeholder: "subnet-0abc (optional)"},
	{Name: "filter_availability_zone", Type: core.ConnectionTypeString, Label: "Filter by Availability Zone", Placeholder: "e.g. eu-west-2a (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "instances", Type: core.ConnectionTypeObject, Label: "Instances"},
	{Name: "instance_ids", Type: core.ConnectionTypeObject, Label: "Instance IDs"},
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
	if filters := buildFilters(inputs); len(filters) > 0 {
		in.Filters = filters
	}

	var instances []map[string]interface{}
	var ids []string
	paginator := ec2.NewDescribeInstancesPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, res := range page.Reservations {
			for _, inst := range res.Instances {
				instances = append(instances, summariseInstance(inst))
				ids = append(ids, aws.ToString(inst.InstanceId))
			}
		}
	}

	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Found %d EC2 instance(s)", len(instances)),
		"instances":    instances,
		"instance_ids": ids,
		"count":        len(instances),
	}, nil
}

// buildFilters assembles the EC2 DescribeInstances filters from the optional
// filter inputs. Tag rows become "tag:<Key>" = <Value> (or "tag-key" = <Key>
// when the value is blank); the named filters map to their EC2 filter names.
// EC2 ANDs all filters together.
func buildFilters(inputs []*core.Connection) []types.Filter {
	var filters []types.Filter

	if conn := core.FindConnection("filter_tags", inputs); conn != nil {
		for _, kv := range conn.KeyValuePairs() {
			k := strings.TrimSpace(kv.Key)
			if k == "" {
				continue
			}
			if v := strings.TrimSpace(kv.Value); v != "" {
				filters = append(filters, types.Filter{Name: aws.String("tag:" + k), Values: []string{v}})
			} else {
				filters = append(filters, types.Filter{Name: aws.String("tag-key"), Values: []string{k}})
			}
		}
	}

	// State is multi-select: multiple states OR together within the one filter.
	if states := awscommon.InputStrings("filter_state", inputs); len(states) > 0 {
		filters = append(filters, types.Filter{Name: aws.String("instance-state-name"), Values: states})
	}

	named := []struct{ input, filter string }{
		{"filter_instance_type", "instance-type"},
		{"filter_vpc_id", "vpc-id"},
		{"filter_subnet_id", "subnet-id"},
		{"filter_availability_zone", "availability-zone"},
	}
	for _, n := range named {
		if v := strings.TrimSpace(awscommon.InputString(n.input, inputs)); v != "" {
			filters = append(filters, types.Filter{Name: aws.String(n.filter), Values: []string{v}})
		}
	}

	return filters
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
