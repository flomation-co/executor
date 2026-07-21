// Package aws_s3_put_bucket_lifecycle_configuration sets an S3 bucket's
// lifecycle configuration from a curated JSON rule set.
package aws_s3_put_bucket_lifecycle_configuration

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
	Name         = "AWS S3 Put Bucket Lifecycle"
	Description  = "Set a bucket's lifecycle rules (expiry/transition) from a curated JSON list."
	Website      = "https://www.flomation.co"
	Icon         = "bucket+clock"
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
	{Name: "rules", Type: core.ConnectionTypeString, Label: "Rules (JSON array)", Placeholder: `[{"id":"expire-logs","prefix":"logs/","status":"Enabled","expiration_days":90}]`, Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "rule_count", Type: core.ConnectionTypeInteger, Label: "Rules Applied"},
}

// lifecycleRule is the curated subset of a lifecycle rule this v1 action
// supports. Advanced fields (tags, size filters, noncurrent versions, abort
// multipart) are intentionally omitted.
type lifecycleRule struct {
	ID                     string `json:"id"`
	Prefix                 string `json:"prefix"`
	Status                 string `json:"status"`
	ExpirationDays         int32  `json:"expiration_days"`
	TransitionDays         int32  `json:"transition_days"`
	TransitionStorageClass string `json:"transition_storage_class"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	bucket := awscommon.InputString("bucket", inputs)
	if bucket == "" {
		return nil, fmt.Errorf("bucket is required")
	}
	rulesRaw := strings.TrimSpace(awscommon.InputString("rules", inputs))
	if rulesRaw == "" {
		return nil, fmt.Errorf("rules JSON array is required")
	}

	var parsed []lifecycleRule
	if err := json.Unmarshal([]byte(rulesRaw), &parsed); err != nil {
		return nil, fmt.Errorf("rules must be a JSON array: %w", err)
	}
	if len(parsed) == 0 {
		return nil, fmt.Errorf("at least one lifecycle rule is required")
	}

	rules := make([]s3types.LifecycleRule, 0, len(parsed))
	for i, r := range parsed {
		status := s3types.ExpirationStatusEnabled
		if strings.EqualFold(strings.TrimSpace(r.Status), "Disabled") {
			status = s3types.ExpirationStatusDisabled
		}
		rule := s3types.LifecycleRule{
			Status: status,
			Filter: &s3types.LifecycleRuleFilter{Prefix: aws.String(r.Prefix)},
		}
		if r.ID != "" {
			rule.ID = aws.String(r.ID)
		}
		if r.ExpirationDays > 0 {
			rule.Expiration = &s3types.LifecycleExpiration{Days: aws.Int32(r.ExpirationDays)}
		}
		if r.TransitionDays > 0 && r.TransitionStorageClass != "" {
			rule.Transitions = []s3types.Transition{{
				Days:         aws.Int32(r.TransitionDays),
				StorageClass: s3types.TransitionStorageClass(r.TransitionStorageClass),
			}}
		}
		if rule.Expiration == nil && len(rule.Transitions) == 0 {
			return nil, fmt.Errorf("rule %d must set expiration_days or transition_days+transition_storage_class", i)
		}
		rules = append(rules, rule)
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := awsS3.NewFromConfig(cfg)

	_, err = client.PutBucketLifecycleConfiguration(ctx, &awsS3.PutBucketLifecycleConfigurationInput{
		Bucket:                 aws.String(bucket),
		LifecycleConfiguration: &s3types.BucketLifecycleConfiguration{Rules: rules},
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Applied %d lifecycle rule(s) to %s", len(rules), bucket),
		"rule_count":  len(rules),
	}, nil
}
