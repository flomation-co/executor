// Package aws_ec2_detach_volume detaches an EBS volume from an instance.
package aws_ec2_detach_volume

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
	Name         = "AWS EC2 Detach Volume"
	Description  = "Detach an EBS volume from an instance."
	Website      = "https://www.flomation.co"
	Icon         = "hard-drive+arrow-down"
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
	{Name: "volume_id", Type: core.ConnectionTypeString, Label: "Volume ID", Placeholder: "vol-0abc123", Required: true},
	{Name: "instance_id", Type: core.ConnectionTypeString, Label: "Instance ID (optional)", Placeholder: "i-0abc123"},
	{Name: "device", Type: core.ConnectionTypeString, Label: "Device Name (optional)", Placeholder: "/dev/sdf"},
	{Name: "force", Type: core.ConnectionTypeBoolean, Label: "Force Detach"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "state", Type: core.ConnectionTypeString, Label: "State"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	volumeID := awscommon.InputString("volume_id", inputs)
	if volumeID == "" {
		return nil, fmt.Errorf("volume id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.DetachVolumeInput{VolumeId: aws.String(volumeID)}
	if id := awscommon.InputString("instance_id", inputs); id != "" {
		in.InstanceId = aws.String(id)
	}
	if d := awscommon.InputString("device", inputs); d != "" {
		in.Device = aws.String(d)
	}
	if awscommon.InputBool("force", inputs) {
		in.Force = aws.Bool(true)
	}

	out, err := client.DetachVolume(ctx, in)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Detached volume %s", volumeID),
		"state":       string(out.State),
	}, nil
}
