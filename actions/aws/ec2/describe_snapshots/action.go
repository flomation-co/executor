// Package aws_ec2_describe_snapshots lists EBS snapshots.
package aws_ec2_describe_snapshots

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
	Name         = "AWS EC2 Describe Snapshots"
	Description  = "List EBS snapshots by id or owner, with volume, size, state and progress."
	Website      = "https://www.flomation.co"
	Icon         = "floppy-disk"
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
	{Name: "snapshot_ids", Type: core.ConnectionTypeString, Label: "Snapshot IDs", Placeholder: "Comma-separated (optional)"},
	{Name: "owners", Type: core.ConnectionTypeString, Label: "Owners", Placeholder: "e.g. self (optional; blank lists all visible)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "snapshots", Type: core.ConnectionTypeObject, Label: "Snapshots"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.DescribeSnapshotsInput{}
	if ids := awscommon.InputStrings("snapshot_ids", inputs); len(ids) > 0 {
		in.SnapshotIds = ids
	}
	if owners := awscommon.InputStrings("owners", inputs); len(owners) > 0 {
		in.OwnerIds = owners
	}

	var snapshots []map[string]interface{}
	paginator := ec2.NewDescribeSnapshotsPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, s := range page.Snapshots {
			snapshots = append(snapshots, map[string]interface{}{
				"snapshot_id": aws.ToString(s.SnapshotId),
				"volume_id":   aws.ToString(s.VolumeId),
				"size_gib":    aws.ToInt32(s.VolumeSize),
				"state":       string(s.State),
				"progress":    aws.ToString(s.Progress),
				"description": aws.ToString(s.Description),
				"encrypted":   aws.ToBool(s.Encrypted),
			})
		}
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d snapshot(s)", len(snapshots)),
		"snapshots":   snapshots,
		"count":       len(snapshots),
	}, nil
}
