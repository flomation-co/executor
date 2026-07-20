// Package aws_ec2_describe_images lists AMIs (Amazon Machine Images).
package aws_ec2_describe_images

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
	Name         = "AWS EC2 Describe Images"
	Description  = "List AMIs by id or owner, with name, state, architecture and creation date."
	Website      = "https://www.flomation.co"
	Icon         = "image"
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
	{Name: "image_ids", Type: core.ConnectionTypeString, Label: "AMI IDs", Placeholder: "Comma-separated (optional)"},
	{Name: "owners", Type: core.ConnectionTypeString, Label: "Owners", Placeholder: "e.g. self, amazon, or an account id (optional)"},
	{Name: "filter_name", Type: core.ConnectionTypeString, Label: "Filter by Name", Placeholder: "AMI name; wildcards allowed e.g. ubuntu-*/22.04* (optional)"},
	{Name: "filter_state", Type: core.ConnectionTypeMultiSelect, Label: "Filter by State", Placeholder: "Select one or more; none = any state", Options: []core.ConnectionOption{
		{Name: "Available", Value: "available"},
		{Name: "Pending", Value: "pending"},
		{Name: "Failed", Value: "failed"},
	}},
	{Name: "filter_architecture", Type: core.ConnectionTypeMultiSelect, Label: "Filter by Architecture", Placeholder: "Select one or more; none = any", Options: []core.ConnectionOption{
		{Name: "x86_64", Value: "x86_64"},
		{Name: "arm64", Value: "arm64"},
		{Name: "i386", Value: "i386"},
	}},
	{Name: "filter_is_public", Type: core.ConnectionTypeString, Label: "Filter by Visibility", Options: []core.ConnectionOption{
		{Name: "Any", Value: ""},
		{Name: "Public only", Value: "true"},
		{Name: "Private only", Value: "false"},
	}},
	{Name: "filter_platform", Type: core.ConnectionTypeString, Label: "Filter by Platform", Placeholder: "e.g. windows — blank includes Linux/UNIX (optional)"},
	{Name: "filter_tags", Type: core.ConnectionTypeKeyValueArray, Label: "Filter by Tags", Placeholder: "Only return images with these tags (blank Value matches any value for that key)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "images", Type: core.ConnectionTypeObject, Label: "Images"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.DescribeImagesInput{}
	if ids := awscommon.InputStrings("image_ids", inputs); len(ids) > 0 {
		in.ImageIds = ids
	}
	if owners := awscommon.InputStrings("owners", inputs); len(owners) > 0 {
		in.Owners = owners
	}
	if filters := awscommon.BuildEC2Filters(inputs, []awscommon.FilterSpec{
		{Input: "filter_name", Filter: "name"},
		{Input: "filter_state", Filter: "state", Multi: true},
		{Input: "filter_architecture", Filter: "architecture", Multi: true},
		{Input: "filter_is_public", Filter: "is-public"},
		{Input: "filter_platform", Filter: "platform"},
	}); len(filters) > 0 {
		in.Filters = filters
	}

	out, err := client.DescribeImages(ctx, in)
	if err != nil {
		return nil, err
	}

	var images []map[string]interface{}
	for _, img := range out.Images {
		images = append(images, summariseImage(img))
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d image(s)", len(images)),
		"images":      images,
		"count":       len(images),
	}, nil
}

func summariseImage(img types.Image) map[string]interface{} {
	return map[string]interface{}{
		"image_id":      aws.ToString(img.ImageId),
		"name":          aws.ToString(img.Name),
		"description":   aws.ToString(img.Description),
		"state":         string(img.State),
		"architecture":  string(img.Architecture),
		"platform":      aws.ToString(img.PlatformDetails),
		"owner_id":      aws.ToString(img.OwnerId),
		"public":        aws.ToBool(img.Public),
		"creation_date": aws.ToString(img.CreationDate),
	}
}
