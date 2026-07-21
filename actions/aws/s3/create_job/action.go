// Package aws_s3_create_job creates an S3 Batch Operations job.
package aws_s3_create_job

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
	Name         = "AWS S3 Create Batch Job"
	Description  = "Create an S3 Batch Operations job; operation/manifest/report are JSON objects."
	Website      = "https://www.flomation.co"
	Icon         = "layer-group+plus"
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
	{Name: "role_arn", Type: core.ConnectionTypeString, Label: "Job IAM Role ARN", Placeholder: "arn:aws:iam::<account>:role/BatchOpsRole", Required: true},
	{Name: "priority", Type: core.ConnectionTypeInteger, Label: "Priority", Placeholder: "10", Required: true},
	{Name: "operation", Type: core.ConnectionTypeString, Label: "Operation (JSON)", Placeholder: `{"S3PutObjectCopy":{"TargetResource":"arn:aws:s3:::dest-bucket"}}`, Required: true},
	{Name: "manifest", Type: core.ConnectionTypeString, Label: "Manifest (JSON)", Placeholder: `{"Spec":{"Format":"S3BatchOperations_CSV_20180820","Fields":["Bucket","Key"]},"Location":{"ObjectArn":"arn:aws:s3:::bucket/manifest.csv","ETag":"<etag>"}}`, Required: true},
	{Name: "report", Type: core.ConnectionTypeString, Label: "Report (JSON)", Placeholder: `{"Enabled":true,"Bucket":"arn:aws:s3:::report-bucket","Format":"Report_CSV_20180820","Prefix":"reports/","ReportScope":"AllTasks"}`, Required: true},
	{Name: "confirmation_required", Type: core.ConnectionTypeBoolean, Label: "Confirmation Required"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description (optional)", Placeholder: "Nightly copy of new objects"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "job_id", Type: core.ConnectionTypeString, Label: "Job ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	roleArn := strings.TrimSpace(awscommon.InputString("role_arn", inputs))
	if roleArn == "" {
		return nil, fmt.Errorf("role_arn is required")
	}
	priority, ok := awscommon.InputInt("priority", inputs)
	if !ok {
		return nil, fmt.Errorf("priority is required")
	}

	operationRaw := strings.TrimSpace(awscommon.InputString("operation", inputs))
	if operationRaw == "" {
		return nil, fmt.Errorf("operation JSON is required")
	}
	var operation s3ctltypes.JobOperation
	if err := json.Unmarshal([]byte(operationRaw), &operation); err != nil {
		return nil, fmt.Errorf("operation must be a JSON object: %w", err)
	}

	manifestRaw := strings.TrimSpace(awscommon.InputString("manifest", inputs))
	if manifestRaw == "" {
		return nil, fmt.Errorf("manifest JSON is required")
	}
	var manifest s3ctltypes.JobManifest
	if err := json.Unmarshal([]byte(manifestRaw), &manifest); err != nil {
		return nil, fmt.Errorf("manifest must be a JSON object: %w", err)
	}

	reportRaw := strings.TrimSpace(awscommon.InputString("report", inputs))
	if reportRaw == "" {
		return nil, fmt.Errorf("report JSON is required")
	}
	var report s3ctltypes.JobReport
	if err := json.Unmarshal([]byte(reportRaw), &report); err != nil {
		return nil, fmt.Errorf("report must be a JSON object: %w", err)
	}

	description := strings.TrimSpace(awscommon.InputString("description", inputs))

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	accountID, err := awscommon.ResolveAccountID(ctx, cfg, inputs)
	if err != nil {
		return nil, err
	}
	client := s3control.NewFromConfig(cfg)

	// ClientRequestToken is an idempotency token — derive a stable-ish value from
	// the account, priority and description so a retried Execute doesn't create a
	// duplicate job.
	token := fmt.Sprintf("flomation-%s-%d-%s", accountID, priority, description)
	if len(token) > 64 {
		token = token[:64]
	}

	in := &s3control.CreateJobInput{
		AccountId:          aws.String(accountID),
		ClientRequestToken: aws.String(token),
		RoleArn:            aws.String(roleArn),
		Priority:           aws.Int32(int32(priority)),
		Operation:          &operation,
		Manifest:           &manifest,
		Report:             &report,
	}
	if awscommon.InputBool("confirmation_required", inputs) {
		in.ConfirmationRequired = aws.Bool(true)
	}
	if description != "" {
		in.Description = aws.String(description)
	}

	out, err := client.CreateJob(ctx, in)
	if err != nil {
		return nil, err
	}

	jobID := aws.ToString(out.JobId)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created S3 Batch Operations job %s", jobID),
		"job_id":      jobID,
	}, nil
}
