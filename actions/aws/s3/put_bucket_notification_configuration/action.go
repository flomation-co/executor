// Package aws_s3_put_bucket_notification_configuration sets a bucket's event
// notification configuration.
package aws_s3_put_bucket_notification_configuration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsS3 "github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS S3 Put Bucket Notification"
	Description  = "Set an S3 bucket's event notifications (v1: one queue/topic/lambda destination)."
	Website      = "https://www.flomation.co"
	Icon         = "bell+pen"
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
	{Name: "bucket", Type: core.ConnectionTypeString, Label: "Bucket", Placeholder: "my-bucket", Required: true},
	{Name: "destination_type", Type: core.ConnectionTypeString, Label: "Destination Type", Required: true, Options: []core.ConnectionOption{
		{Name: "SQS Queue", Value: "queue"},
		{Name: "SNS Topic", Value: "topic"},
		{Name: "Lambda Function", Value: "lambda"},
	}},
	{Name: "destination_arn", Type: core.ConnectionTypeString, Label: "Destination ARN", Placeholder: "arn:aws:sqs:eu-west-2:123456789012:my-queue", Required: true},
	{Name: "events", Type: core.ConnectionTypeString, Label: "Events (comma-separated)", Placeholder: "s3:ObjectCreated:*, s3:ObjectRemoved:*", Required: true},
	{Name: "configuration", Type: core.ConnectionTypeString, Label: "Configuration JSON (optional override)", Placeholder: `{"QueueConfigurations":[{"QueueArn":"...","Events":["s3:ObjectCreated:*"]}]}`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "bucket", Type: core.ConnectionTypeString, Label: "Bucket"},
}

// notificationConfig mirrors the shape of s3types.NotificationConfiguration for
// the optional raw-JSON override. It is a LOCAL struct so the manifest generator
// need not see the SDK type; only the fields we support are decoded.
type notificationConfig struct {
	QueueConfigurations []struct {
		QueueArn string   `json:"QueueArn"`
		Events   []string `json:"Events"`
		Id       string   `json:"Id"`
	} `json:"QueueConfigurations"`
	TopicConfigurations []struct {
		TopicArn string   `json:"TopicArn"`
		Events   []string `json:"Events"`
		Id       string   `json:"Id"`
	} `json:"TopicConfigurations"`
	LambdaFunctionConfigurations []struct {
		LambdaFunctionArn string   `json:"LambdaFunctionArn"`
		Events            []string `json:"Events"`
		Id                string   `json:"Id"`
	} `json:"LambdaFunctionConfigurations"`
}

func toEvents(list []string) []s3types.Event {
	out := make([]s3types.Event, 0, len(list))
	for _, e := range list {
		if t := strings.TrimSpace(e); t != "" {
			out = append(out, s3types.Event(t))
		}
	}
	return out
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	bucket := awscommon.InputString("bucket", inputs)
	if bucket == "" {
		return nil, fmt.Errorf("bucket is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := awsS3.NewFromConfig(cfg)

	nc := &s3types.NotificationConfiguration{}

	if raw := strings.TrimSpace(awscommon.InputString("configuration", inputs)); raw != "" {
		var lc notificationConfig
		if err := json.Unmarshal([]byte(raw), &lc); err != nil {
			return nil, fmt.Errorf("parse configuration JSON: %w", err)
		}
		for _, q := range lc.QueueConfigurations {
			cfgItem := s3types.QueueConfiguration{QueueArn: aws.String(q.QueueArn), Events: toEvents(q.Events)}
			if q.Id != "" {
				cfgItem.Id = aws.String(q.Id)
			}
			nc.QueueConfigurations = append(nc.QueueConfigurations, cfgItem)
		}
		for _, tpc := range lc.TopicConfigurations {
			cfgItem := s3types.TopicConfiguration{TopicArn: aws.String(tpc.TopicArn), Events: toEvents(tpc.Events)}
			if tpc.Id != "" {
				cfgItem.Id = aws.String(tpc.Id)
			}
			nc.TopicConfigurations = append(nc.TopicConfigurations, cfgItem)
		}
		for _, l := range lc.LambdaFunctionConfigurations {
			cfgItem := s3types.LambdaFunctionConfiguration{LambdaFunctionArn: aws.String(l.LambdaFunctionArn), Events: toEvents(l.Events)}
			if l.Id != "" {
				cfgItem.Id = aws.String(l.Id)
			}
			nc.LambdaFunctionConfigurations = append(nc.LambdaFunctionConfigurations, cfgItem)
		}
	} else {
		destType := awscommon.InputString("destination_type", inputs)
		destArn := awscommon.InputString("destination_arn", inputs)
		if destType == "" {
			return nil, fmt.Errorf("destination_type is required")
		}
		if destArn == "" {
			return nil, fmt.Errorf("destination_arn is required")
		}
		events := toEvents(strings.Split(awscommon.InputString("events", inputs), ","))
		if len(events) == 0 {
			return nil, fmt.Errorf("at least one event is required")
		}
		switch destType {
		case "queue":
			nc.QueueConfigurations = []s3types.QueueConfiguration{{QueueArn: aws.String(destArn), Events: events}}
		case "topic":
			nc.TopicConfigurations = []s3types.TopicConfiguration{{TopicArn: aws.String(destArn), Events: events}}
		case "lambda":
			nc.LambdaFunctionConfigurations = []s3types.LambdaFunctionConfiguration{{LambdaFunctionArn: aws.String(destArn), Events: events}}
		default:
			return nil, fmt.Errorf("unsupported destination_type %q", destType)
		}
	}

	_, err = client.PutBucketNotificationConfiguration(ctx, &awsS3.PutBucketNotificationConfigurationInput{
		Bucket:                    aws.String(bucket),
		NotificationConfiguration: nc,
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Set notification configuration on bucket %s", bucket),
		"bucket":      bucket,
	}, nil
}
