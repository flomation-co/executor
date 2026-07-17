// Package aws_ec2_describe_volumes lists EBS volumes.
package aws_ec2_describe_volumes

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
	Name         = "AWS EC2 Describe Volumes"
	Description  = "List EBS volumes with size, state, type, AZ and attachments."
	Website      = "https://www.flomation.co"
	Icon         = "database"
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
	{Name: "volume_ids", Type: core.ConnectionTypeString, Label: "Volume IDs", Placeholder: "Comma-separated; blank for all (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "volumes", Type: core.ConnectionTypeObject, Label: "Volumes"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.DescribeVolumesInput{}
	if ids := awscommon.InputStrings("volume_ids", inputs); len(ids) > 0 {
		in.VolumeIds = ids
	}

	var volumes []map[string]interface{}
	paginator := ec2.NewDescribeVolumesPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, v := range page.Volumes {
			var attachedTo []string
			for _, a := range v.Attachments {
				attachedTo = append(attachedTo, aws.ToString(a.InstanceId))
			}
			volumes = append(volumes, map[string]interface{}{
				"volume_id":         aws.ToString(v.VolumeId),
				"size_gib":          aws.ToInt32(v.Size),
				"state":             string(v.State),
				"volume_type":       string(v.VolumeType),
				"availability_zone": aws.ToString(v.AvailabilityZone),
				"encrypted":         aws.ToBool(v.Encrypted),
				"attached_to":       attachedTo,
			})
		}
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d volume(s)", len(volumes)),
		"volumes":     volumes,
		"count":       len(volumes),
	}, nil
}
