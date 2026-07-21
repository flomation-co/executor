// Package aws_ec2_describe_instance_types lists EC2 instance types with their
// vCPU and memory sizing.
package aws_ec2_describe_instance_types

import (
	"context"
	"encoding/json"
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
	Name         = "AWS EC2 Describe Instance Types"
	Description  = "List EC2 instance types with their vCPU and memory sizing."
	Website      = "https://www.flomation.co"
	Icon         = "server+list"
	Date         = "21/07/2026"
	Type         = core.ActionTypeAction

	// maxTypes caps the returned list so an unfiltered "all types" query doesn't
	// produce a huge payload.
	maxTypes = 200
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
	{Name: "instance_types", Type: core.ConnectionTypeString, Label: "Instance Types", Placeholder: "Comma-separated; leave blank for all (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "instance_types_info", Type: core.ConnectionTypeString, Label: "Instance Types (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.DescribeInstanceTypesInput{}
	if raw := awscommon.InputStrings("instance_types", inputs); len(raw) > 0 {
		its := make([]types.InstanceType, 0, len(raw))
		for _, r := range raw {
			its = append(its, types.InstanceType(r))
		}
		in.InstanceTypes = its
	}

	var infos []map[string]interface{}
	truncated := false
	paginator := ec2.NewDescribeInstanceTypesPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, it := range page.InstanceTypes {
			if len(infos) >= maxTypes {
				truncated = true
				break
			}
			infos = append(infos, summariseType(it))
		}
		if truncated {
			break
		}
	}

	infosJSON, err := json.Marshal(infos)
	if err != nil {
		return nil, err
	}

	summary := fmt.Sprintf("Found %d instance type(s)", len(infos))
	if truncated {
		summary = fmt.Sprintf("Returning the first %d instance type(s) (result truncated)", len(infos))
	}

	return map[string]interface{}{
		"tool_result":         summary,
		"instance_types_info": string(infosJSON),
		"count":               len(infos),
	}, nil
}

func summariseType(it types.InstanceTypeInfo) map[string]interface{} {
	m := map[string]interface{}{
		"instance_type": string(it.InstanceType),
	}
	if it.VCpuInfo != nil {
		m["vcpus"] = aws.ToInt32(it.VCpuInfo.DefaultVCpus)
	}
	if it.MemoryInfo != nil {
		m["memory_mib"] = aws.ToInt64(it.MemoryInfo.SizeInMiB)
	}
	return m
}
