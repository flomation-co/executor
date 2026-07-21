// Package aws_rds_create_event_subscription creates an RDS event notification
// subscription delivered to an SNS topic.
package aws_rds_create_event_subscription

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Create Event Subscription"
	Description  = "Create an RDS event notification subscription delivered to an SNS topic."
	Website      = "https://www.flomation.co"
	Icon         = "bell+plus"
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
	{Name: "subscription_name", Type: core.ConnectionTypeString, Label: "Subscription Name", Placeholder: "my-rds-events", Required: true},
	{Name: "sns_topic_arn", Type: core.ConnectionTypeString, Label: "SNS Topic ARN", Placeholder: "arn:aws:sns:eu-west-2:123456789012:my-topic", Required: true},
	{Name: "source_type", Type: core.ConnectionTypeString, Label: "Source Type (optional)", Placeholder: "e.g. db-instance"},
	{Name: "source_ids", Type: core.ConnectionTypeString, Label: "Source IDs (optional)", Placeholder: "Comma-separated identifiers"},
	{Name: "event_categories", Type: core.ConnectionTypeString, Label: "Event Categories (optional)", Placeholder: "Comma-separated categories"},
	{Name: "enabled", Type: core.ConnectionTypeBoolean, Label: "Enabled"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "subscription", Type: core.ConnectionTypeObject, Label: "Event Subscription"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	name := awscommon.InputString("subscription_name", inputs)
	if name == "" {
		return nil, fmt.Errorf("subscription name is required")
	}
	topic := awscommon.InputString("sns_topic_arn", inputs)
	if topic == "" {
		return nil, fmt.Errorf("sns topic arn is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.CreateEventSubscriptionInput{
		SubscriptionName: aws.String(name),
		SnsTopicArn:      aws.String(topic),
		Enabled:          aws.Bool(awscommon.InputBool("enabled", inputs)),
	}
	if st := awscommon.InputString("source_type", inputs); st != "" {
		in.SourceType = aws.String(st)
	}
	if ids := awscommon.InputStrings("source_ids", inputs); len(ids) > 0 {
		in.SourceIds = ids
	}
	if cats := awscommon.InputStrings("event_categories", inputs); len(cats) > 0 {
		in.EventCategories = cats
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.Tags = tags
	}

	out, err := client.CreateEventSubscription(ctx, in)
	if err != nil {
		return nil, err
	}

	var subscription map[string]interface{}
	if s := out.EventSubscription; s != nil {
		subscription = map[string]interface{}{
			"subscription_name": aws.ToString(s.CustSubscriptionId),
			"arn":               aws.ToString(s.EventSubscriptionArn),
			"sns_topic_arn":     aws.ToString(s.SnsTopicArn),
			"source_type":       aws.ToString(s.SourceType),
			"status":            aws.ToString(s.Status),
			"enabled":           aws.ToBool(s.Enabled),
			"source_ids":        s.SourceIdsList,
			"event_categories":  s.EventCategoriesList,
			"customer_aws_id":   aws.ToString(s.CustomerAwsId),
			"created_at":        aws.ToString(s.SubscriptionCreationTime),
		}
	}

	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Created event subscription %q", name),
		"subscription": subscription,
	}, nil
}

func buildTags(inputs []*core.Connection) []rdstypes.Tag {
	conn := core.FindConnection("tags", inputs)
	if conn == nil {
		return nil
	}
	var tags []rdstypes.Tag
	for _, kv := range conn.KeyValuePairs() {
		k := strings.TrimSpace(kv.Key)
		if k == "" {
			continue
		}
		tags = append(tags, rdstypes.Tag{Key: aws.String(k), Value: aws.String(kv.Value)})
	}
	return tags
}
