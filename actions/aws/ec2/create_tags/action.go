// Package aws_ec2_create_tags adds or overwrites tags on EC2 resources.
package aws_ec2_create_tags

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
	Name         = "AWS EC2 Create Tags"
	Description  = "Add or overwrite tags on EC2 resources (instances, volumes, etc)."
	Website      = "https://www.flomation.co"
	Icon         = "hashtag+plus"
	Date         = "17/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "aws_access_key", Type: core.ConnectionTypeSecret, Label: "AWS Access Key", Required: true},
	{Name: "aws_secret_key", Type: core.ConnectionTypeSecret, Label: "AWS Secret Key", Required: true},
	{Name: "aws_region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "eu-west-2", Required: true},
	{Name: "aws_session_token", Type: core.ConnectionTypeSecret, Label: "Session Token (optional)"},
	{Name: "assume_role_arn", Type: core.ConnectionTypeString, Label: "Assume Role ARN (optional)", Placeholder: "arn:aws:iam::123456789012:role/MyRole"},
	{Name: "external_id", Type: core.ConnectionTypeString, Label: "Assume Role External ID (optional)", Placeholder: "Must match the External ID in the role's trust policy"},
	{Name: "resource_ids", Type: core.ConnectionTypeString, Label: "Resource IDs", Placeholder: "Comma-separated, e.g. i-0abc,vol-0def", Required: true},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Tags", Placeholder: "Key=Value pairs, e.g. Name=web,Env=prod", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "tagged", Type: core.ConnectionTypeInteger, Label: "Resources Tagged"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	ids := awscommon.InputStrings("resource_ids", inputs)
	if len(ids) == 0 {
		return nil, fmt.Errorf("at least one resource id is required")
	}

	tags := parseTags(awscommon.InputString("tags", inputs))
	if len(tags) == 0 {
		return nil, fmt.Errorf("at least one Key=Value tag is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	if _, err := client.CreateTags(ctx, &ec2.CreateTagsInput{Resources: ids, Tags: tags}); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Applied %d tag(s) to %d resource(s)", len(tags), len(ids)),
		"tagged":      len(ids),
	}, nil
}

// parseTags reads a "Key=Value,Key2=Value2" string into SDK tags.
func parseTags(raw string) []types.Tag {
	var tags []types.Tag
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		k, v, ok := strings.Cut(pair, "=")
		k = strings.TrimSpace(k)
		if !ok || k == "" {
			continue
		}
		tags = append(tags, types.Tag{Key: aws.String(k), Value: aws.String(strings.TrimSpace(v))})
	}
	return tags
}
