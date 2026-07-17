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
	{Name: "aws_access_key", Type: core.ConnectionTypeSecret, Label: "AWS Access Key", Required: true},
	{Name: "aws_secret_key", Type: core.ConnectionTypeSecret, Label: "AWS Secret Key", Required: true},
	{Name: "aws_region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "eu-west-2", Required: true},
	{Name: "aws_session_token", Type: core.ConnectionTypeSecret, Label: "Session Token (optional)"},
	{Name: "assume_role_arn", Type: core.ConnectionTypeString, Label: "Assume Role ARN (optional)", Placeholder: "arn:aws:iam::123456789012:role/MyRole"},
	{Name: "image_ids", Type: core.ConnectionTypeString, Label: "AMI IDs", Placeholder: "Comma-separated (optional)"},
	{Name: "owners", Type: core.ConnectionTypeString, Label: "Owners", Placeholder: "e.g. self, amazon, or an account id (optional)"},
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
