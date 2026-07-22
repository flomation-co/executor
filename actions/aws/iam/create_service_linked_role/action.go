// Package aws_iam_create_service_linked_role creates an IAM service-linked role.
package aws_iam_create_service_linked_role

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS IAM Create Service-Linked Role"
	Description  = "Create an IAM service-linked role for an AWS service principal."
	Website      = "https://www.flomation.co"
	Icon         = "shield-halved+plus"
	Date         = "22/07/2026"
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
	{Name: "aws_service_name", Type: core.ConnectionTypeString, Label: "AWS Service Name", Placeholder: "elasticbeanstalk.amazonaws.com", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description (optional)"},
	{Name: "custom_suffix", Type: core.ConnectionTypeString, Label: "Custom Suffix (optional)", Placeholder: "-1"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "role_name", Type: core.ConnectionTypeString, Label: "Role Name"},
	{Name: "arn", Type: core.ConnectionTypeString, Label: "ARN"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	serviceName := awscommon.InputString("aws_service_name", inputs)
	if serviceName == "" {
		return nil, fmt.Errorf("AWS service name is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := iam.NewFromConfig(cfg)

	in := &iam.CreateServiceLinkedRoleInput{
		AWSServiceName: aws.String(serviceName),
	}
	if d := strings.TrimSpace(awscommon.InputString("description", inputs)); d != "" {
		in.Description = aws.String(d)
	}
	if s := strings.TrimSpace(awscommon.InputString("custom_suffix", inputs)); s != "" {
		in.CustomSuffix = aws.String(s)
	}

	out, err := client.CreateServiceLinkedRole(ctx, in)
	if err != nil {
		return nil, err
	}

	var roleName, arn string
	if out.Role != nil {
		roleName = aws.ToString(out.Role.RoleName)
		arn = aws.ToString(out.Role.Arn)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created service-linked role %s (%s)", roleName, arn),
		"role_name":   roleName,
		"arn":         arn,
	}, nil
}
