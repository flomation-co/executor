// Package aws_rds_delete_event_subscription deletes an RDS event notification
// subscription.
package aws_rds_delete_event_subscription

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
	Name         = "AWS RDS Delete Event Subscription"
	Description  = "Delete an RDS event notification subscription."
	Website      = "https://www.flomation.co"
	Icon         = "bell+trash"
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

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	out, err := client.DeleteEventSubscription(ctx, &rds.DeleteEventSubscriptionInput{SubscriptionName: aws.String(name)})
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
		"tool_result":  fmt.Sprintf("Deleted event subscription %q", name),
		"subscription": subscription,
	}, nil
}
