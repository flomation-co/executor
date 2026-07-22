// Package aws_iam_get_role retrieves an IAM role.
package aws_iam_get_role

import (
	"context"
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS IAM Get Role"
	Description  = "Retrieve an IAM role, including its trust policy document."
	Website      = "https://www.flomation.co"
	Icon         = "shield-halved+magnifying-glass"
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
	{Name: "role_name", Type: core.ConnectionTypeString, Label: "Role Name", Placeholder: "my-service-role", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "role_name", Type: core.ConnectionTypeString, Label: "Role Name"},
	{Name: "role_id", Type: core.ConnectionTypeString, Label: "Role ID"},
	{Name: "arn", Type: core.ConnectionTypeString, Label: "ARN"},
	{Name: "assume_role_policy_document", Type: core.ConnectionTypeString, Label: "Trust Policy Document (JSON)"},
	{Name: "max_session_duration", Type: core.ConnectionTypeInteger, Label: "Max Session Duration (seconds)"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	roleName := awscommon.InputString("role_name", inputs)
	if roleName == "" {
		return nil, fmt.Errorf("role name is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := iam.NewFromConfig(cfg)

	out, err := client.GetRole(ctx, &iam.GetRoleInput{RoleName: aws.String(roleName)})
	if err != nil {
		return nil, err
	}

	var roleID, arn, policyDoc string
	var maxSession int32
	if out.Role != nil {
		roleID = aws.ToString(out.Role.RoleId)
		arn = aws.ToString(out.Role.Arn)
		maxSession = aws.ToInt32(out.Role.MaxSessionDuration)
		raw := aws.ToString(out.Role.AssumeRolePolicyDocument)
		if decoded, derr := url.QueryUnescape(raw); derr == nil {
			policyDoc = decoded
		} else {
			policyDoc = raw
		}
	}

	return map[string]interface{}{
		"tool_result":                 fmt.Sprintf("Retrieved IAM role %s (%s)", roleName, arn),
		"role_name":                   roleName,
		"role_id":                     roleID,
		"arn":                         arn,
		"assume_role_policy_document": policyDoc,
		"max_session_duration":        int(maxSession),
	}, nil
}
