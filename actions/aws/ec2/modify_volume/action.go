// Package aws_ec2_modify_volume modifies an existing EBS volume.
package aws_ec2_modify_volume

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS EC2 Modify Volume"
	Description  = "Modify an EBS volume's size, type, IOPS or throughput."
	Website      = "https://www.flomation.co"
	Icon         = "hard-drive+pen"
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
	{Name: "size", Type: core.ConnectionTypeInteger, Label: "Size (GiB, optional)", Placeholder: "Larger than current"},
	{Name: "volume_type", Type: core.ConnectionTypeString, Label: "Volume Type (optional)", Options: []core.ConnectionOption{
		{Name: "General Purpose SSD (gp3)", Value: "gp3"},
		{Name: "General Purpose SSD (gp2)", Value: "gp2"},
		{Name: "Provisioned IOPS SSD (io1)", Value: "io1"},
		{Name: "Provisioned IOPS SSD (io2)", Value: "io2"},
		{Name: "Throughput Optimised HDD (st1)", Value: "st1"},
		{Name: "Cold HDD (sc1)", Value: "sc1"},
		{Name: "Magnetic (standard)", Value: "standard"},
	}},
	{Name: "iops", Type: core.ConnectionTypeInteger, Label: "IOPS (optional)"},
	{Name: "throughput", Type: core.ConnectionTypeInteger, Label: "Throughput MiB/s (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "volume_id", Type: core.ConnectionTypeString, Label: "Volume ID"},
	{Name: "modification_state", Type: core.ConnectionTypeString, Label: "Modification State"},
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

	in := &ec2.ModifyVolumeInput{VolumeId: aws.String(volumeID)}
	if v, ok := awscommon.InputInt("size", inputs); ok {
		in.Size = aws.Int32(int32(v))
	}
	if vt := awscommon.InputString("volume_type", inputs); vt != "" {
		in.VolumeType = ec2types.VolumeType(vt)
	}
	if v, ok := awscommon.InputInt("iops", inputs); ok {
		in.Iops = aws.Int32(int32(v))
	}
	if v, ok := awscommon.InputInt("throughput", inputs); ok {
		in.Throughput = aws.Int32(int32(v))
	}

	out, err := client.ModifyVolume(ctx, in)
	if err != nil {
		return nil, err
	}

	var modState string
	if out.VolumeModification != nil {
		modState = string(out.VolumeModification.ModificationState)
	}
	return map[string]interface{}{
		"tool_result":        fmt.Sprintf("Requested modification of volume %s (%s)", volumeID, modState),
		"volume_id":          volumeID,
		"modification_state": modState,
	}, nil
}
