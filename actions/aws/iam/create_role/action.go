// Package aws_iam_create_role creates an IAM role.
package aws_iam_create_role

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS IAM Create Role"
	Description  = "Create a new IAM role with a trust policy, plus optional tags and boundary."
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
	{Name: "role_name", Type: core.ConnectionTypeString, Label: "Role Name", Placeholder: "my-service-role", Required: true},
	{Name: "assume_role_policy_document", Type: core.ConnectionTypeString, Label: "Trust Policy Document (JSON)", Placeholder: `{"Version":"2012-10-17","Statement":[...]}`, Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description (optional)"},
	{Name: "path", Type: core.ConnectionTypeString, Label: "Path (optional)", Placeholder: "/division_abc/"},
	{Name: "max_session_duration", Type: core.ConnectionTypeInteger, Label: "Max Session Duration (seconds, optional)", Placeholder: "3600"},
	{Name: "permissions_boundary", Type: core.ConnectionTypeString, Label: "Permissions Boundary ARN (optional)", Placeholder: "arn:aws:iam::<account>:policy/Boundary"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "role_name", Type: core.ConnectionTypeString, Label: "Role Name"},
	{Name: "role_id", Type: core.ConnectionTypeString, Label: "Role ID"},
	{Name: "arn", Type: core.ConnectionTypeString, Label: "ARN"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	roleName := awscommon.InputString("role_name", inputs)
	if roleName == "" {
		return nil, fmt.Errorf("role name is required")
	}
	policyDoc := awscommon.InputString("assume_role_policy_document", inputs)
	if strings.TrimSpace(policyDoc) == "" {
		return nil, fmt.Errorf("assume role policy document is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := iam.NewFromConfig(cfg)

	in := &iam.CreateRoleInput{
		RoleName:                 aws.String(roleName),
		AssumeRolePolicyDocument: aws.String(policyDoc),
	}
	if d := strings.TrimSpace(awscommon.InputString("description", inputs)); d != "" {
		in.Description = aws.String(d)
	}
	if p := strings.TrimSpace(awscommon.InputString("path", inputs)); p != "" {
		in.Path = aws.String(p)
	}
	if b := strings.TrimSpace(awscommon.InputString("permissions_boundary", inputs)); b != "" {
		in.PermissionsBoundary = aws.String(b)
	}
	if d, ok := awscommon.InputInt("max_session_duration", inputs); ok {
		in.MaxSessionDuration = aws.Int32(int32(d))
	}
	if conn := core.FindConnection("tags", inputs); conn != nil {
		for _, kv := range conn.KeyValuePairs() {
			k := strings.TrimSpace(kv.Key)
			if k == "" {
				continue
			}
			in.Tags = append(in.Tags, iamtypes.Tag{Key: aws.String(k), Value: aws.String(kv.Value)})
		}
	}

	out, err := client.CreateRole(ctx, in)
	if err != nil {
		return nil, err
	}

	var roleID, arn string
	if out.Role != nil {
		roleID = aws.ToString(out.Role.RoleId)
		arn = aws.ToString(out.Role.Arn)
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created IAM role %s (%s)", roleName, arn),
		"role_name":   roleName,
		"role_id":     roleID,
		"arn":         arn,
	}, nil
}
