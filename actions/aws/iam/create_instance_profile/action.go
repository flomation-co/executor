// Package aws_iam_create_instance_profile creates an IAM instance profile.
package aws_iam_create_instance_profile

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
	Name         = "AWS IAM Create Instance Profile"
	Description  = "Create an IAM instance profile with optional path and tags."
	Website      = "https://www.flomation.co"
	Icon         = "id-badge+plus"
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
	{Name: "instance_profile_name", Type: core.ConnectionTypeString, Label: "Instance Profile Name", Placeholder: "my-instance-profile", Required: true},
	{Name: "path", Type: core.ConnectionTypeString, Label: "Path (optional)", Placeholder: "/division_abc/"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "instance_profile_name", Type: core.ConnectionTypeString, Label: "Instance Profile Name"},
	{Name: "arn", Type: core.ConnectionTypeString, Label: "ARN"},
	{Name: "instance_profile_id", Type: core.ConnectionTypeString, Label: "Instance Profile ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	profileName := awscommon.InputString("instance_profile_name", inputs)
	if profileName == "" {
		return nil, fmt.Errorf("instance profile name is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := iam.NewFromConfig(cfg)

	in := &iam.CreateInstanceProfileInput{InstanceProfileName: aws.String(profileName)}
	if p := strings.TrimSpace(awscommon.InputString("path", inputs)); p != "" {
		in.Path = aws.String(p)
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

	out, err := client.CreateInstanceProfile(ctx, in)
	if err != nil {
		return nil, err
	}

	var arn, profileID string
	if out.InstanceProfile != nil {
		arn = aws.ToString(out.InstanceProfile.Arn)
		profileID = aws.ToString(out.InstanceProfile.InstanceProfileId)
	}
	return map[string]interface{}{
		"tool_result":           fmt.Sprintf("Created instance profile %s (%s)", profileName, arn),
		"instance_profile_name": profileName,
		"arn":                   arn,
		"instance_profile_id":   profileID,
	}, nil
}
