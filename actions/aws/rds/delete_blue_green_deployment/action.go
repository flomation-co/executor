// Package aws_rds_delete_blue_green_deployment deletes an RDS blue/green
// deployment, optionally tearing down the green (target) environment.
package aws_rds_delete_blue_green_deployment

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Delete Blue/Green Deployment"
	Description  = "Delete a blue/green deployment, optionally removing the green environment."
	Website      = "https://www.flomation.co"
	Icon         = "code-branch+trash"
	Date         = "20/07/2026"
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
	{Name: "blue_green_deployment_identifier", Type: core.ConnectionTypeString, Label: "Blue/Green Deployment Identifier", Placeholder: "bgd-xxxxxxxxxxxx", Required: true},
	{Name: "delete_target", Type: core.ConnectionTypeBoolean, Label: "Delete Green Environment"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "deployment", Type: core.ConnectionTypeObject, Label: "Blue/Green Deployment"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	id := awscommon.InputString("blue_green_deployment_identifier", inputs)
	if id == "" {
		return nil, fmt.Errorf("blue/green deployment identifier is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.DeleteBlueGreenDeploymentInput{
		BlueGreenDeploymentIdentifier: aws.String(id),
	}
	if awscommon.InputBool("delete_target", inputs) {
		in.DeleteTarget = aws.Bool(true)
	}

	out, err := client.DeleteBlueGreenDeployment(ctx, in)
	if err != nil {
		return nil, err
	}

	dep := out.BlueGreenDeployment
	deployment := map[string]interface{}{
		"blue_green_deployment_identifier": aws.ToString(dep.BlueGreenDeploymentIdentifier),
		"name":                             aws.ToString(dep.BlueGreenDeploymentName),
		"source":                           aws.ToString(dep.Source),
		"target":                           aws.ToString(dep.Target),
		"status":                           aws.ToString(dep.Status),
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Deleting blue/green deployment %q (status: %s)", id, aws.ToString(dep.Status)),
		"deployment":  deployment,
	}, nil
}
