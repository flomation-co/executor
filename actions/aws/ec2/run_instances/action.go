// Package aws_ec2_run_instances launches new EC2 instances from an AMI.
package aws_ec2_run_instances

import (
	"context"
	"encoding/base64"
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
	Name         = "AWS EC2 Run Instances"
	Description  = "Launch new EC2 instances from an AMI, with type, key pair, subnet and security groups."
	Website      = "https://www.flomation.co"
	Icon         = "server+plus"
	Date         = "17/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Required: true, Options: []core.ConnectionOption{
		{Name: "Access Keys", Value: "keys"},
		{Name: "Assume Role (cross-account)", Value: "assume_role"},
	}},
	{Name: "aws_access_key", Type: core.ConnectionTypeSecret, Label: "AWS Access Key", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "aws_secret_key", Type: core.ConnectionTypeSecret, Label: "AWS Secret Key", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "aws_region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "eu-west-2", Required: true},
	{Name: "aws_session_token", Type: core.ConnectionTypeSecret, Label: "Session Token (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "assume_role_arn", Type: core.ConnectionTypeString, Label: "Role ARN to Assume", Placeholder: "arn:aws:iam::<your-account>:role/FlomationAccess", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"assume_role"}}},
	{Name: "external_id", Type: core.ConnectionTypeString, Label: "Assume Role External ID (optional)", Placeholder: "Must match the External ID in the role's trust policy", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"assume_role"}}},
	{Name: "image_id", Type: core.ConnectionTypeString, Label: "AMI ID", Placeholder: "ami-0abc123", Required: true},
	{Name: "instance_type", Type: core.ConnectionTypeString, Label: "Instance Type", Placeholder: "t3.micro", Required: true},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count", Placeholder: "1"},
	{Name: "key_name", Type: core.ConnectionTypeString, Label: "Key Pair Name", Placeholder: "Optional SSH key pair"},
	{Name: "subnet_id", Type: core.ConnectionTypeString, Label: "Subnet ID", Placeholder: "Optional subnet"},
	{Name: "security_group_ids", Type: core.ConnectionTypeString, Label: "Security Group IDs", Placeholder: "Comma-separated (optional)"},
	{Name: "name_tag", Type: core.ConnectionTypeString, Label: "Name Tag", Placeholder: "Sets the Name tag (optional)"},
	{Name: "user_data", Type: core.ConnectionTypeText, Label: "User Data", Placeholder: "Startup script, plain text (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "instance_ids", Type: core.ConnectionTypeObject, Label: "Instance IDs"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	count := int32(1)
	if c := core.FindConnection("count", inputs); c != nil {
		if n := c.Number(); n != nil && *n > 0 {
			count = int32(*n)
		}
	}

	in := &ec2.RunInstancesInput{
		ImageId:      aws.String(awscommon.InputString("image_id", inputs)),
		InstanceType: types.InstanceType(awscommon.InputString("instance_type", inputs)),
		MinCount:     aws.Int32(count),
		MaxCount:     aws.Int32(count),
	}
	if v := awscommon.InputString("key_name", inputs); v != "" {
		in.KeyName = aws.String(v)
	}
	if v := awscommon.InputString("subnet_id", inputs); v != "" {
		in.SubnetId = aws.String(v)
	}
	if sgs := awscommon.InputStrings("security_group_ids", inputs); len(sgs) > 0 {
		in.SecurityGroupIds = sgs
	}
	if ud := awscommon.InputString("user_data", inputs); ud != "" {
		// The SDK expects User Data pre-base64-encoded.
		in.UserData = aws.String(base64.StdEncoding.EncodeToString([]byte(ud)))
	}
	if name := awscommon.InputString("name_tag", inputs); name != "" {
		in.TagSpecifications = []types.TagSpecification{{
			ResourceType: types.ResourceTypeInstance,
			Tags:         []types.Tag{{Key: aws.String("Name"), Value: aws.String(name)}},
		}}
	}

	out, err := client.RunInstances(ctx, in)
	if err != nil {
		return nil, err
	}

	var ids []string
	for _, inst := range out.Instances {
		ids = append(ids, aws.ToString(inst.InstanceId))
	}

	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Launched %d instance(s): %v", len(ids), ids),
		"instance_ids": ids,
		"count":        len(ids),
	}, nil
}
