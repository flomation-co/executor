// Package aws_s3_list_jobs lists S3 Batch Operations jobs.
package aws_s3_list_jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3control"
	s3ctltypes "github.com/aws/aws-sdk-go-v2/service/s3control/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS S3 List Batch Jobs"
	Description  = "List S3 Batch Operations jobs, optionally filtered by status."
	Website      = "https://www.flomation.co"
	Icon         = "layer-group+list"
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
	{Name: "job_statuses", Type: core.ConnectionTypeString, Label: "Filter by Statuses (optional)", Placeholder: "Active,Complete,Failed (comma-separated)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "jobs", Type: core.ConnectionTypeString, Label: "Jobs (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	accountID, err := awscommon.ResolveAccountID(ctx, cfg, inputs)
	if err != nil {
		return nil, err
	}
	client := s3control.NewFromConfig(cfg)

	var statuses []s3ctltypes.JobStatus
	for _, s := range awscommon.InputStrings("job_statuses", inputs) {
		statuses = append(statuses, s3ctltypes.JobStatus(s))
	}

	in := &s3control.ListJobsInput{AccountId: aws.String(accountID)}
	if len(statuses) > 0 {
		in.JobStatuses = statuses
	}

	jobs := make([]map[string]interface{}, 0)
	paginator := s3control.NewListJobsPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, j := range page.Jobs {
			jobs = append(jobs, map[string]interface{}{
				"job_id":    aws.ToString(j.JobId),
				"status":    string(j.Status),
				"priority":  j.Priority,
				"operation": string(j.Operation),
			})
		}
	}

	jobsJSON := "[]"
	if b, err := json.Marshal(jobs); err == nil {
		jobsJSON = string(b)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d S3 Batch Operations job(s)", len(jobs)),
		"jobs":        jobsJSON,
		"count":       len(jobs),
	}, nil
}

// keep strings imported for potential future filtering helpers
var _ = strings.TrimSpace
