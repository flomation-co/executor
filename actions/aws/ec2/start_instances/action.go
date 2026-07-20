// Package aws_ec2_start_instances starts one or more stopped EC2 instances.
package aws_ec2_start_instances

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
	Name         = "AWS EC2 Start Instances"
	Description  = "Start one or more stopped EC2 instances."
	Website      = "https://www.flomation.co"
	Icon         = "server+play"
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
	{Name: "instance_ids", Type: core.ConnectionTypeString, Label: "Instance IDs", Placeholder: "Comma-separated, e.g. i-0abc,i-0def", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "state_changes", Type: core.ConnectionTypeObject, Label: "State Changes"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	ids := awscommon.InputStrings("instance_ids", inputs)
	if len(ids) == 0 {
		return nil, fmt.Errorf("at least one instance id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	out, err := client.StartInstances(ctx, &ec2.StartInstancesInput{InstanceIds: ids})
	if err != nil {
		return nil, err
	}

	var changes []map[string]interface{}
	for _, c := range out.StartingInstances {
		changes = append(changes, map[string]interface{}{
			"instance_id":    aws.ToString(c.InstanceId),
			"previous_state": stateName(c.PreviousState),
			"current_state":  stateName(c.CurrentState),
		})
	}

	return map[string]interface{}{
		"tool_result":   fmt.Sprintf("Started %d instance(s)", len(changes)),
		"state_changes": changes,
	}, nil
}

func stateName(s *types.InstanceState) string {
	if s == nil {
		return ""
	}
	return string(s.Name)
}
