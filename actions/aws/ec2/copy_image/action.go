// Package aws_ec2_copy_image copies an AMI into this region.
package aws_ec2_copy_image

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
	Name         = "AWS EC2 Copy Image"
	Description  = "Copy an AMI from a source region into this action's region."
	Website      = "https://www.flomation.co"
	Icon         = "image+copy"
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
	{Name: "source_region", Type: core.ConnectionTypeString, Label: "Source Region", Placeholder: "us-east-1", Required: true},
	{Name: "source_image_id", Type: core.ConnectionTypeString, Label: "Source Image (AMI) ID", Placeholder: "ami-0abc123", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "New Image Name", Placeholder: "my-app-golden-copy", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "Optional"},
	{Name: "encrypted", Type: core.ConnectionTypeBoolean, Label: "Encrypt Copy"},
	{Name: "kms_key_id", Type: core.ConnectionTypeString, Label: "KMS Key ID (optional)", Placeholder: "alias/aws/ebs or a key ARN"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "image_id", Type: core.ConnectionTypeString, Label: "New Image (AMI) ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	sourceRegion := awscommon.InputString("source_region", inputs)
	if sourceRegion == "" {
		return nil, fmt.Errorf("source region is required")
	}
	sourceImageID := awscommon.InputString("source_image_id", inputs)
	if sourceImageID == "" {
		return nil, fmt.Errorf("source image id is required")
	}
	name := awscommon.InputString("name", inputs)
	if name == "" {
		return nil, fmt.Errorf("new image name is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.CopyImageInput{
		SourceRegion:  aws.String(sourceRegion),
		SourceImageId: aws.String(sourceImageID),
		Name:          aws.String(name),
	}
	if d := awscommon.InputString("description", inputs); d != "" {
		in.Description = aws.String(d)
	}
	if awscommon.InputBool("encrypted", inputs) {
		in.Encrypted = aws.Bool(true)
	}
	if k := awscommon.InputString("kms_key_id", inputs); k != "" {
		in.KmsKeyId = aws.String(k)
	}

	out, err := client.CopyImage(ctx, in)
	if err != nil {
		return nil, err
	}

	imageID := aws.ToString(out.ImageId)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Copied image %s from %s to %s (%s)", sourceImageID, sourceRegion, imageID, name),
		"image_id":    imageID,
	}, nil
}
