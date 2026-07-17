// Package aws_ec2_stop_instances stops one or more running EC2 instances.
package aws_ec2_stop_instances

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
	Name         = "AWS EC2 Stop Instances"
	Description  = "Stop one or more running EC2 instances."
	Website      = "https://www.flomation.co"
	Icon         = "server+stop"
	Date         = "17/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "aws_access_key", Type: core.ConnectionTypeSecret, Label: "AWS Access Key", Required: true},
	{Name: "aws_secret_key", Type: core.ConnectionTypeSecret, Label: "AWS Secret Key", Required: true},
	{Name: "aws_region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "eu-west-2", Required: true},
	{Name: "aws_session_token", Type: core.ConnectionTypeSecret, Label: "Session Token (optional)"},
	{Name: "assume_role_arn", Type: core.ConnectionTypeString, Label: "Assume Role ARN (optional)", Placeholder: "arn:aws:iam::123456789012:role/MyRole"},
	{Name: "instance_ids", Type: core.ConnectionTypeString, Label: "Instance IDs", Placeholder: "Comma-separated, e.g. i-0abc,i-0def", Required: true},
	{Name: "force", Type: core.ConnectionTypeBoolean, Label: "Force Stop", Placeholder: "Force stop without a clean shutdown"},
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

	in := &ec2.StopInstancesInput{InstanceIds: ids}
	if force := core.FindConnection("force", inputs); force != nil {
		if b := force.Boolean(); b != nil && *b {
			in.Force = aws.Bool(true)
		}
	}

	out, err := client.StopInstances(ctx, in)
	if err != nil {
		return nil, err
	}

	var changes []map[string]interface{}
	for _, c := range out.StoppingInstances {
		changes = append(changes, map[string]interface{}{
			"instance_id":    aws.ToString(c.InstanceId),
			"previous_state": stateName(c.PreviousState),
			"current_state":  stateName(c.CurrentState),
		})
	}

	return map[string]interface{}{
		"tool_result":   fmt.Sprintf("Stopped %d instance(s)", len(changes)),
		"state_changes": changes,
	}, nil
}

func stateName(s *types.InstanceState) string {
	if s == nil {
		return ""
	}
	return string(s.Name)
}
