// Package aws_ec2_describe_instance_status reports EC2 instance and system
// status checks.
package aws_ec2_describe_instance_status

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
	Name         = "AWS EC2 Describe Instance Status"
	Description  = "Report EC2 instance state and system/instance status checks."
	Website      = "https://www.flomation.co"
	Icon         = "server+magnifying-glass"
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
	{Name: "instance_ids", Type: core.ConnectionTypeString, Label: "Instance IDs", Placeholder: "Comma-separated; leave blank for all (optional)"},
	{Name: "include_all_instances", Type: core.ConnectionTypeBoolean, Label: "Include All Instances", Placeholder: "Include non-running instances (default: running only)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "statuses", Type: core.ConnectionTypeString, Label: "Statuses (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.DescribeInstanceStatusInput{}
	if ids := awscommon.InputStrings("instance_ids", inputs); len(ids) > 0 {
		in.InstanceIds = ids
	}
	if awscommon.InputBool("include_all_instances", inputs) {
		in.IncludeAllInstances = aws.Bool(true)
	}

	var statuses []map[string]interface{}
	paginator := ec2.NewDescribeInstanceStatusPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, s := range page.InstanceStatuses {
			statuses = append(statuses, summariseStatus(s))
		}
	}

	statusesJSON, err := json.Marshal(statuses)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Retrieved status for %d instance(s)", len(statuses)),
		"statuses":    string(statusesJSON),
		"count":       len(statuses),
	}, nil
}

func summariseStatus(s types.InstanceStatus) map[string]interface{} {
	m := map[string]interface{}{
		"instance_id": aws.ToString(s.InstanceId),
	}
	if s.InstanceState != nil {
		m["instance_state"] = string(s.InstanceState.Name)
	}
	if s.SystemStatus != nil {
		m["system_status"] = string(s.SystemStatus.Status)
	}
	if s.InstanceStatus != nil {
		m["instance_status"] = string(s.InstanceStatus.Status)
	}
	if s.AvailabilityZone != nil {
		m["availability_zone"] = aws.ToString(s.AvailabilityZone)
	}
	return m
}
