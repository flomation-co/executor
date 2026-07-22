// Package aws_iam_tag_instance_profile adds or updates tags on an IAM instance profile.
package aws_iam_tag_instance_profile

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
	Name         = "AWS IAM Tag Instance Profile"
	Description  = "Add or update tags on an IAM instance profile."
	Website      = "https://www.flomation.co"
	Icon         = "id-badge+tag"
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
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "instance_profile_name", Type: core.ConnectionTypeString, Label: "Instance Profile Name"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	profileName := awscommon.InputString("instance_profile_name", inputs)
	if profileName == "" {
		return nil, fmt.Errorf("instance profile name is required")
	}

	var tags []iamtypes.Tag
	if conn := core.FindConnection("tags", inputs); conn != nil {
		for _, kv := range conn.KeyValuePairs() {
			k := strings.TrimSpace(kv.Key)
			if k == "" {
				continue
			}
			tags = append(tags, iamtypes.Tag{Key: aws.String(k), Value: aws.String(kv.Value)})
		}
	}
	if len(tags) == 0 {
		return nil, fmt.Errorf("at least one tag is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := iam.NewFromConfig(cfg)

	_, err = client.TagInstanceProfile(ctx, &iam.TagInstanceProfileInput{
		InstanceProfileName: aws.String(profileName),
		Tags:                tags,
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result":           fmt.Sprintf("Tagged IAM instance profile %s with %d tag(s)", profileName, len(tags)),
		"instance_profile_name": profileName,
	}, nil
}
