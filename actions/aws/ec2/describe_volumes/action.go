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
	{Name: "filter_status", Type: core.ConnectionTypeMultiSelect, Label: "Filter by Status", Placeholder: "Select one or more; none = any status", Options: []core.ConnectionOption{
		{Name: "Available", Value: "available"},
		{Name: "In use", Value: "in-use"},
		{Name: "Creating", Value: "creating"},
		{Name: "Deleting", Value: "deleting"},
		{Name: "Deleted", Value: "deleted"},
		{Name: "Error", Value: "error"},
	}},
	{Name: "filter_volume_type", Type: core.ConnectionTypeMultiSelect, Label: "Filter by Volume Type", Placeholder: "Select one or more; none = any type", Options: []core.ConnectionOption{
		{Name: "gp3", Value: "gp3"},
		{Name: "gp2", Value: "gp2"},
		{Name: "io1", Value: "io1"},
		{Name: "io2", Value: "io2"},
		{Name: "st1", Value: "st1"},
		{Name: "sc1", Value: "sc1"},
		{Name: "standard (magnetic)", Value: "standard"},
	}},
	{Name: "filter_availability_zone", Type: core.ConnectionTypeString, Label: "Filter by Availability Zone", Placeholder: "e.g. eu-west-2a (optional)"},
	{Name: "filter_instance_id", Type: core.ConnectionTypeString, Label: "Filter by Attached Instance ID", Placeholder: "i-0abc — volumes attached to this instance (optional)"},
	{Name: "filter_tags", Type: core.ConnectionTypeKeyValueArray, Label: "Filter by Tags", Placeholder: "Only return volumes with these tags (blank Value matches any value for that key)"},
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
	if filters := awscommon.BuildEC2Filters(inputs, []awscommon.FilterSpec{
		{Input: "filter_status", Filter: "status", Multi: true},
		{Input: "filter_volume_type", Filter: "volume-type", Multi: true},
		{Input: "filter_availability_zone", Filter: "availability-zone"},
		{Input: "filter_instance_id", Filter: "attachment.instance-id"},
	}); len(filters) > 0 {
		in.Filters = filters
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
