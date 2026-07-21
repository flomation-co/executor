// Package aws_s3_describe_job describes an S3 Batch Operations job.
package aws_s3_describe_job

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3control"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS S3 Describe Batch Job"
	Description  = "Retrieve the status, priority and progress of an S3 Batch Operations job."
	Website      = "https://www.flomation.co"
	Icon         = "layer-group+magnifying-glass"
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
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "AWS Account ID", Placeholder: "12-digit account ID; leave blank to auto-detect from the credential"},
	{Name: "job_id", Type: core.ConnectionTypeString, Label: "Job ID", Placeholder: "the batch job ID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "priority", Type: core.ConnectionTypeInteger, Label: "Priority"},
	{Name: "progress", Type: core.ConnectionTypeString, Label: "Progress (JSON)"},
	{Name: "job", Type: core.ConnectionTypeString, Label: "Job (JSON)"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	jobID := strings.TrimSpace(awscommon.InputString("job_id", inputs))
	if jobID == "" {
		return nil, fmt.Errorf("job_id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	accountID, err := awscommon.ResolveAccountID(ctx, cfg, inputs)
	if err != nil {
		return nil, err
	}
	client := s3control.NewFromConfig(cfg)

	out, err := client.DescribeJob(ctx, &s3control.DescribeJobInput{
		AccountId: aws.String(accountID),
		JobId:     aws.String(jobID),
	})
	if err != nil {
		return nil, err
	}

	status := ""
	var priority int32
	progressJSON := ""
	jobJSON := ""
	if out.Job != nil {
		status = string(out.Job.Status)
		priority = out.Job.Priority
		if out.Job.ProgressSummary != nil {
			if b, err := json.Marshal(out.Job.ProgressSummary); err == nil {
				progressJSON = string(b)
			}
		}
		job := map[string]interface{}{
			"job_id":   aws.ToString(out.Job.JobId),
			"job_arn":  aws.ToString(out.Job.JobArn),
			"status":   status,
			"priority": priority,
			"role_arn": aws.ToString(out.Job.RoleArn),
		}
		if out.Job.Description != nil {
			job["description"] = aws.ToString(out.Job.Description)
		}
		if b, err := json.Marshal(job); err == nil {
			jobJSON = string(b)
		}
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Job %s is %s (priority %d)", jobID, status, priority),
		"status":      status,
		"priority":    int(priority),
		"progress":    progressJSON,
		"job":         jobJSON,
	}, nil
}
