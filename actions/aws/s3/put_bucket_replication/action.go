// Package aws_s3_put_bucket_replication sets an S3 bucket's cross-region
// replication configuration from a curated JSON rule set.
package aws_s3_put_bucket_replication

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
	Name         = "AWS S3 Put Bucket Replication"
	Description  = "Set a bucket's replication rules (curated subset) from a JSON list. Requires an IAM role ARN."
	Website      = "https://www.flomation.co"
	Icon         = "copy+pen"
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
	{Name: "bucket", Type: core.ConnectionTypeString, Label: "Bucket", Placeholder: "my-source-bucket", Required: true},
	{Name: "role", Type: core.ConnectionTypeString, Label: "Replication IAM Role ARN", Placeholder: "arn:aws:iam::<account>:role/replication-role", Required: true},
	{Name: "rules", Type: core.ConnectionTypeString, Label: "Rules (JSON array)", Placeholder: `[{"id":"repl-1","status":"Enabled","priority":1,"prefix":"","dest_bucket":"arn:aws:s3:::dest-bucket","dest_storage_class":"STANDARD"}]`, Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "rule_count", Type: core.ConnectionTypeInteger, Label: "Rules Applied"},
}

// replicationRule is the curated subset of a replication rule this v1 action
// supports. Advanced fields (tags/And filters, delete-marker replication,
// replica ownership, RTC/metrics, encryption) are intentionally omitted.
type replicationRule struct {
	ID               string `json:"id"`
	Status           string `json:"status"`
	Priority         int32  `json:"priority"`
	Prefix           string `json:"prefix"`
	DestBucket       string `json:"dest_bucket"`
	DestStorageClass string `json:"dest_storage_class"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	bucket := awscommon.InputString("bucket", inputs)
	if bucket == "" {
		return nil, fmt.Errorf("bucket is required")
	}
	role := strings.TrimSpace(awscommon.InputString("role", inputs))
	if role == "" {
		return nil, fmt.Errorf("role (replication IAM role ARN) is required")
	}
	rulesRaw := strings.TrimSpace(awscommon.InputString("rules", inputs))
	if rulesRaw == "" {
		return nil, fmt.Errorf("rules JSON array is required")
	}

	var parsed []replicationRule
	if err := json.Unmarshal([]byte(rulesRaw), &parsed); err != nil {
		return nil, fmt.Errorf("rules must be a JSON array: %w", err)
	}
	if len(parsed) == 0 {
		return nil, fmt.Errorf("at least one replication rule is required")
	}

	rules := make([]s3types.ReplicationRule, 0, len(parsed))
	for i, r := range parsed {
		if strings.TrimSpace(r.DestBucket) == "" {
			return nil, fmt.Errorf("rule %d must set dest_bucket (destination bucket ARN)", i)
		}
		status := s3types.ReplicationRuleStatusEnabled
		if strings.EqualFold(strings.TrimSpace(r.Status), "Disabled") {
			status = s3types.ReplicationRuleStatusDisabled
		}
		rule := s3types.ReplicationRule{
			Status:   status,
			Priority: aws.Int32(r.Priority),
			Filter:   &s3types.ReplicationRuleFilter{Prefix: aws.String(r.Prefix)},
			Destination: &s3types.Destination{
				Bucket: aws.String(r.DestBucket),
			},
		}
		if r.ID != "" {
			rule.ID = aws.String(r.ID)
		}
		if r.DestStorageClass != "" {
			rule.Destination.StorageClass = s3types.StorageClass(r.DestStorageClass)
		}
		rules = append(rules, rule)
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := awsS3.NewFromConfig(cfg)

	_, err = client.PutBucketReplication(ctx, &awsS3.PutBucketReplicationInput{
		Bucket: aws.String(bucket),
		ReplicationConfiguration: &s3types.ReplicationConfiguration{
			Role:  aws.String(role),
			Rules: rules,
		},
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Applied %d replication rule(s) to %s", len(rules), bucket),
		"rule_count":  len(rules),
	}, nil
}
