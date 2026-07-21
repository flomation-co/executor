// Package aws_ec2_modify_instance_attribute modifies a curated subset of an EC2
// instance's attributes. AWS ModifyInstanceAttribute accepts only one attribute
// per request, so each provided field is applied in its own call.
package aws_ec2_modify_instance_attribute

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS EC2 Modify Instance Attribute"
	Description  = "Change an EC2 instance's type or its termination, source/dest and EBS-optimised flags."
	Website      = "https://www.flomation.co"
	Icon         = "server+pen"
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
	{Name: "instance_id", Type: core.ConnectionTypeString, Label: "Instance ID", Placeholder: "i-0abc123", Required: true},
	{Name: "instance_type", Type: core.ConnectionTypeString, Label: "Instance Type", Placeholder: "e.g. t3.large (optional; instance must be stopped)"},
	{Name: "disable_api_termination", Type: core.ConnectionTypeBoolean, Label: "Disable API Termination", Placeholder: "Enable termination protection (leave blank to leave unchanged)"},
	{Name: "source_dest_check", Type: core.ConnectionTypeBoolean, Label: "Source/Dest Check", Placeholder: "Enable source/destination checks (leave blank to leave unchanged)"},
	{Name: "ebs_optimized", Type: core.ConnectionTypeBoolean, Label: "EBS Optimised", Placeholder: "Enable EBS optimisation (leave blank to leave unchanged)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
}

// hasValue reports whether an input carries a non-empty raw value. Used for the
// optional boolean flags so we only apply a flag the user actually set (an unset
// checkbox must leave the attribute untouched, not force it to false).
func hasValue(name string, inputs []*core.Connection) bool {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return false
	}
	if s := c.String(); s != nil {
		return strings.TrimSpace(*s) != ""
	}
	return c.Value != nil
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	instanceID := awscommon.InputString("instance_id", inputs)
	if instanceID == "" {
		return nil, fmt.Errorf("instance id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	var applied []string

	if it := awscommon.InputString("instance_type", inputs); it != "" {
		if _, err := client.ModifyInstanceAttribute(ctx, &ec2.ModifyInstanceAttributeInput{
			InstanceId:   aws.String(instanceID),
			InstanceType: &types.AttributeValue{Value: aws.String(it)},
		}); err != nil {
			return nil, err
		}
		applied = append(applied, "instance_type="+it)
	}

	if hasValue("disable_api_termination", inputs) {
		v := awscommon.InputBool("disable_api_termination", inputs)
		if _, err := client.ModifyInstanceAttribute(ctx, &ec2.ModifyInstanceAttributeInput{
			InstanceId:            aws.String(instanceID),
			DisableApiTermination: &types.AttributeBooleanValue{Value: aws.Bool(v)},
		}); err != nil {
			return nil, err
		}
		applied = append(applied, fmt.Sprintf("disable_api_termination=%t", v))
	}

	if hasValue("source_dest_check", inputs) {
		v := awscommon.InputBool("source_dest_check", inputs)
		if _, err := client.ModifyInstanceAttribute(ctx, &ec2.ModifyInstanceAttributeInput{
			InstanceId:      aws.String(instanceID),
			SourceDestCheck: &types.AttributeBooleanValue{Value: aws.Bool(v)},
		}); err != nil {
			return nil, err
		}
		applied = append(applied, fmt.Sprintf("source_dest_check=%t", v))
	}

	if hasValue("ebs_optimized", inputs) {
		v := awscommon.InputBool("ebs_optimized", inputs)
		if _, err := client.ModifyInstanceAttribute(ctx, &ec2.ModifyInstanceAttributeInput{
			InstanceId:   aws.String(instanceID),
			EbsOptimized: &types.AttributeBooleanValue{Value: aws.Bool(v)},
		}); err != nil {
			return nil, err
		}
		applied = append(applied, fmt.Sprintf("ebs_optimized=%t", v))
	}

	if len(applied) == 0 {
		return map[string]interface{}{
			"tool_result": fmt.Sprintf("No attributes provided for %s; nothing changed", instanceID),
		}, nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Modified %s: %s", instanceID, strings.Join(applied, ", ")),
	}, nil
}
