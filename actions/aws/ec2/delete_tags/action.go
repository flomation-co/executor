// Package aws_ec2_delete_tags deletes tags from EC2 resources. When no tags are
// specified, all user-defined tags on the resources are removed.
package aws_ec2_delete_tags

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
	Name         = "AWS EC2 Delete Tags"
	Description  = "Delete tags from EC2 resources (all user tags when none given)."
	Website      = "https://www.flomation.co"
	Icon         = "hashtag+trash"
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
	{Name: "resource_ids", Type: core.ConnectionTypeString, Label: "Resource IDs", Placeholder: "Comma-separated, e.g. i-0abc,vol-0def", Required: true},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags (optional)", Placeholder: "Leave empty to delete all user tags"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "resources", Type: core.ConnectionTypeInteger, Label: "Resources Affected"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	ids := awscommon.InputStrings("resource_ids", inputs)
	if len(ids) == 0 {
		return nil, fmt.Errorf("at least one resource id is required")
	}

	tags := buildTags(inputs)

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.DeleteTagsInput{Resources: ids}
	if len(tags) > 0 {
		in.Tags = tags
	}
	if _, err := client.DeleteTags(ctx, in); err != nil {
		return nil, err
	}

	summary := fmt.Sprintf("Deleted all tags from %d resource(s)", len(ids))
	if len(tags) > 0 {
		summary = fmt.Sprintf("Deleted %d tag(s) from %d resource(s)", len(tags), len(ids))
	}

	return map[string]interface{}{
		"tool_result": summary,
		"resources":   len(ids),
	}, nil
}

// buildTags reads the key/value tag rows into SDK tags, skipping rows with a
// blank key. When no rows are present, DeleteTags removes all user-defined tags.
func buildTags(inputs []*core.Connection) []types.Tag {
	conn := core.FindConnection("tags", inputs)
	if conn == nil {
		return nil
	}
	var tags []types.Tag
	for _, kv := range conn.KeyValuePairs() {
		k := strings.TrimSpace(kv.Key)
		if k == "" {
			continue
		}
		tags = append(tags, types.Tag{Key: aws.String(k), Value: aws.String(strings.TrimSpace(kv.Value))})
	}
	return tags
}
