// Package aws_ec2_create_volume creates a new EBS volume.
package aws_ec2_create_volume

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS EC2 Create Volume"
	Description  = "Create a new EBS volume in an Availability Zone."
	Website      = "https://www.flomation.co"
	Icon         = "hard-drive+plus"
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
	{Name: "availability_zone", Type: core.ConnectionTypeString, Label: "Availability Zone", Placeholder: "eu-west-2a", Required: true},
	{Name: "size", Type: core.ConnectionTypeInteger, Label: "Size (GiB)", Placeholder: "e.g. 100"},
	{Name: "volume_type", Type: core.ConnectionTypeString, Label: "Volume Type", Options: []core.ConnectionOption{
		{Name: "General Purpose SSD (gp3)", Value: "gp3"},
		{Name: "General Purpose SSD (gp2)", Value: "gp2"},
		{Name: "Provisioned IOPS SSD (io1)", Value: "io1"},
		{Name: "Provisioned IOPS SSD (io2)", Value: "io2"},
		{Name: "Throughput Optimised HDD (st1)", Value: "st1"},
		{Name: "Cold HDD (sc1)", Value: "sc1"},
		{Name: "Magnetic (standard)", Value: "standard"},
	}},
	{Name: "iops", Type: core.ConnectionTypeInteger, Label: "IOPS (optional)", Placeholder: "Required for io1/io2, optional for gp3"},
	{Name: "throughput", Type: core.ConnectionTypeInteger, Label: "Throughput MiB/s (optional)", Placeholder: "gp3 only"},
	{Name: "snapshot_id", Type: core.ConnectionTypeString, Label: "Snapshot ID (optional)", Placeholder: "snap-0abc123"},
	{Name: "encrypted", Type: core.ConnectionTypeBoolean, Label: "Encrypted"},
	{Name: "kms_key_id", Type: core.ConnectionTypeString, Label: "KMS Key ID (optional)", Placeholder: "Requires Encrypted"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags", Placeholder: "Add a Key and Value per tag"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "volume_id", Type: core.ConnectionTypeString, Label: "Volume ID"},
	{Name: "state", Type: core.ConnectionTypeString, Label: "State"},
	{Name: "availability_zone", Type: core.ConnectionTypeString, Label: "Availability Zone"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	az := awscommon.InputString("availability_zone", inputs)
	if az == "" {
		return nil, fmt.Errorf("availability zone is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.CreateVolumeInput{AvailabilityZone: aws.String(az)}
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
	if s := awscommon.InputString("snapshot_id", inputs); s != "" {
		in.SnapshotId = aws.String(s)
	}
	if awscommon.InputBool("encrypted", inputs) {
		in.Encrypted = aws.Bool(true)
	}
	if k := awscommon.InputString("kms_key_id", inputs); k != "" {
		in.KmsKeyId = aws.String(k)
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.TagSpecifications = []ec2types.TagSpecification{{
			ResourceType: ec2types.ResourceTypeVolume,
			Tags:         tags,
		}}
	}

	out, err := client.CreateVolume(ctx, in)
	if err != nil {
		return nil, err
	}

	volumeID := aws.ToString(out.VolumeId)
	return map[string]interface{}{
		"tool_result":       fmt.Sprintf("Created volume %s in %s", volumeID, az),
		"volume_id":         volumeID,
		"state":             string(out.State),
		"availability_zone": aws.ToString(out.AvailabilityZone),
	}, nil
}

// buildTags reads the key/value tag rows into SDK tags, skipping blank keys.
func buildTags(inputs []*core.Connection) []ec2types.Tag {
	conn := core.FindConnection("tags", inputs)
	if conn == nil {
		return nil
	}
	var tags []ec2types.Tag
	for _, kv := range conn.KeyValuePairs() {
		k := strings.TrimSpace(kv.Key)
		if k == "" {
			continue
		}
		tags = append(tags, ec2types.Tag{Key: aws.String(k), Value: aws.String(strings.TrimSpace(kv.Value))})
	}
	return tags
}
