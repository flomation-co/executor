// Package aws_s3_update_job_status updates the status of an S3 Batch Operations job.
package aws_s3_update_job_status

import (
	"context"
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
	Name         = "AWS S3 Update Batch Job Status"
	Description  = "Confirm (Ready) or cancel an S3 Batch Operations job."
	Website      = "https://www.flomation.co"
	Icon         = "layer-group+circle-check"
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
	{Name: "job_id", Type: core.ConnectionTypeString, Label: "Job ID", Required: true},
	{Name: "requested_job_status", Type: core.ConnectionTypeString, Label: "Requested Status", Required: true, Options: []core.ConnectionOption{
		{Name: "Ready (confirm & run)", Value: "Ready"},
		{Name: "Cancelled", Value: "Cancelled"},
	}},
	{Name: "status_update_reason", Type: core.ConnectionTypeString, Label: "Reason (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "job_id", Type: core.ConnectionTypeString, Label: "Job ID"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	jobID := strings.TrimSpace(awscommon.InputString("job_id", inputs))
	if jobID == "" {
		return nil, fmt.Errorf("job_id is required")
	}
	status := strings.TrimSpace(awscommon.InputString("requested_job_status", inputs))
	if status == "" {
		return nil, fmt.Errorf("requested_job_status is required")
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

	in := &s3control.UpdateJobStatusInput{
		AccountId:          aws.String(accountID),
		JobId:              aws.String(jobID),
		RequestedJobStatus: s3ctltypes.RequestedJobStatus(status),
	}
	if r := strings.TrimSpace(awscommon.InputString("status_update_reason", inputs)); r != "" {
		in.StatusUpdateReason = aws.String(r)
	}

	out, err := client.UpdateJobStatus(ctx, in)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Job %s status is now %s", aws.ToString(out.JobId), string(out.Status)),
		"job_id":      aws.ToString(out.JobId),
		"status":      string(out.Status),
	}, nil
}
