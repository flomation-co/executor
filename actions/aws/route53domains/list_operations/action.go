// Package aws_route53domains_list_operations lists Route 53 Domains operations,
// optionally filtered by status and submission date.
package aws_route53domains_list_operations

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53domains"
	r53dtypes "github.com/aws/aws-sdk-go-v2/service/route53domains/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Route 53 List Operations"
	Description  = "List Route 53 Domains operations, optionally filtered by status and date."
	Website      = "https://www.flomation.co"
	Icon         = "id-badge+list"
	Date         = "22/07/2026"
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
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status Filter (optional)", Options: []core.ConnectionOption{
		{Name: "Any", Value: ""},
		{Name: "Submitted", Value: "SUBMITTED"},
		{Name: "In Progress", Value: "IN_PROGRESS"},
		{Name: "Error", Value: "ERROR"},
		{Name: "Successful", Value: "SUCCESSFUL"},
		{Name: "Failed", Value: "FAILED"},
	}},
	{Name: "submitted_since", Type: core.ConnectionTypeString, Label: "Submitted Since (RFC3339, optional)", Placeholder: "2026-01-01T00:00:00Z"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "operations", Type: core.ConnectionTypeString, Label: "Operations (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

type operationRow struct {
	OperationID   string `json:"operation_id"`
	Status        string `json:"status"`
	Type          string `json:"type"`
	SubmittedDate string `json:"submitted_date"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	in := &route53domains.ListOperationsInput{}

	if s := strings.TrimSpace(awscommon.InputString("status", inputs)); s != "" {
		in.Status = []r53dtypes.OperationStatus{r53dtypes.OperationStatus(s)}
	}
	if since := strings.TrimSpace(awscommon.InputString("submitted_since", inputs)); since != "" {
		t, err := time.Parse(time.RFC3339, since)
		if err != nil {
			return nil, fmt.Errorf("submitted_since must be an RFC3339 timestamp (e.g. 2026-01-01T00:00:00Z): %w", err)
		}
		in.SubmittedSince = aws.Time(t)
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := route53domains.NewFromConfig(cfg, func(o *route53domains.Options) {
		o.Region = "us-east-1"
	})

	var rows []operationRow
	for {
		out, err := client.ListOperations(ctx, in)
		if err != nil {
			return nil, err
		}
		for _, op := range out.Operations {
			submitted := ""
			if op.SubmittedDate != nil {
				submitted = op.SubmittedDate.UTC().Format("2006-01-02T15:04:05Z")
			}
			rows = append(rows, operationRow{
				OperationID:   aws.ToString(op.OperationId),
				Status:        string(op.Status),
				Type:          string(op.Type),
				SubmittedDate: submitted,
			})
		}
		if out.NextPageMarker == nil || aws.ToString(out.NextPageMarker) == "" {
			break
		}
		in.Marker = out.NextPageMarker
	}

	encoded, err := json.Marshal(rows)
	if err != nil {
		return nil, fmt.Errorf("encoding operations: %w", err)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d operation(s)", len(rows)),
		"operations":  string(encoded),
		"count":       len(rows),
	}, nil
}
