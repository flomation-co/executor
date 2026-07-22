// Package aws_iam_get_group retrieves an IAM group and its members.
package aws_iam_get_group

import (
	"context"
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS IAM Get Group"
	Description  = "Retrieve an IAM group along with the list of its member users."
	Website      = "https://www.flomation.co"
	Icon         = "user-group+magnifying-glass"
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
	{Name: "group_name", Type: core.ConnectionTypeString, Label: "Group Name", Placeholder: "developers", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "group_name", Type: core.ConnectionTypeString, Label: "Group Name"},
	{Name: "arn", Type: core.ConnectionTypeString, Label: "ARN"},
	{Name: "users", Type: core.ConnectionTypeString, Label: "Member User Names (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Member Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	groupName := awscommon.InputString("group_name", inputs)
	if groupName == "" {
		return nil, fmt.Errorf("group name is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := iam.NewFromConfig(cfg)

	var arn string
	users := []string{}
	paginator := iam.NewGetGroupPaginator(client, &iam.GetGroupInput{GroupName: aws.String(groupName)})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		if page.Group != nil && arn == "" {
			arn = aws.ToString(page.Group.Arn)
		}
		for _, u := range page.Users {
			users = append(users, aws.ToString(u.UserName))
		}
	}

	usersJSON, err := json.Marshal(users)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Group %s has %d member(s)", groupName, len(users)),
		"group_name":  groupName,
		"arn":         arn,
		"users":       string(usersJSON),
		"count":       len(users),
	}, nil
}
