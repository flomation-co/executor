// Package aws_ec2_create_snapshot creates an EBS volume snapshot.
package aws_ec2_create_snapshot

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
	Name         = "AWS EC2 Create Snapshot"
	Description  = "Create a point-in-time snapshot of an EBS volume."
	Website      = "https://www.flomation.co"
	Icon         = "floppy-disk+plus"
	Date         = "17/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Required: true, Options: []core.ConnectionOption{
		{Name: "Access Keys", Value: "keys"},
		{Name: "Assume Role (cross-account)", Value: "assume_role"},
	}},
	{Name: "aws_access_key", Type: core.ConnectionTypeSecret, Label: "AWS Access Key", Required: true},
	{Name: "aws_secret_key", Type: core.ConnectionTypeSecret, Label: "AWS Secret Key", Required: true},
	{Name: "aws_region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "eu-west-2", Required: true},
	{Name: "aws_session_token", Type: core.ConnectionTypeSecret, Label: "Session Token (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "assume_role_arn", Type: core.ConnectionTypeString, Label: "Assume Role ARN", Placeholder: "arn:aws:iam::123456789012:role/MyRole", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"assume_role"}}},
	{Name: "external_id", Type: core.ConnectionTypeString, Label: "Assume Role External ID (optional)", Placeholder: "Must match the External ID in the role's trust policy", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"assume_role"}}},
	{Name: "volume_id", Type: core.ConnectionTypeString, Label: "Volume ID", Placeholder: "vol-0abc123", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "Optional"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "snapshot_id", Type: core.ConnectionTypeString, Label: "Snapshot ID"},
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

	in := &ec2.CreateSnapshotInput{VolumeId: aws.String(volumeID)}
	if d := awscommon.InputString("description", inputs); d != "" {
		in.Description = aws.String(d)
	}

	out, err := client.CreateSnapshot(ctx, in)
	if err != nil {
		return nil, err
	}

	snapshotID := aws.ToString(out.SnapshotId)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created snapshot %s of %s", snapshotID, volumeID),
		"snapshot_id": snapshotID,
		"state":       string(out.State),
	}, nil
}
