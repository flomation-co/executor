// Package aws_route53domains_get_operation_detail fetches the status of a
// Route 53 Domains operation by its ID.
package aws_route53domains_get_operation_detail

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53domains"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Route 53 Get Operation Detail"
	Description  = "Fetch the status of a Route 53 Domains operation by its ID."
	Website      = "https://www.flomation.co"
	Icon         = "id-badge+magnifying-glass"
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
	{Name: "operation_id", Type: core.ConnectionTypeString, Label: "Operation ID", Placeholder: "Returned by register/transfer/renew/update actions", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "operation_type", Type: core.ConnectionTypeString, Label: "Operation Type"},
	{Name: "message", Type: core.ConnectionTypeString, Label: "Message"},
	{Name: "domain_name", Type: core.ConnectionTypeString, Label: "Domain Name"},
	{Name: "submitted_date", Type: core.ConnectionTypeString, Label: "Submitted Date"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	operationID := strings.TrimSpace(awscommon.InputString("operation_id", inputs))
	if operationID == "" {
		return nil, fmt.Errorf("operation id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := route53domains.NewFromConfig(cfg, func(o *route53domains.Options) {
		o.Region = "us-east-1"
	})

	out, err := client.GetOperationDetail(ctx, &route53domains.GetOperationDetailInput{
		OperationId: aws.String(operationID),
	})
	if err != nil {
		return nil, err
	}

	status := string(out.Status)
	domainName := aws.ToString(out.DomainName)
	submitted := ""
	if out.SubmittedDate != nil {
		submitted = out.SubmittedDate.UTC().Format("2006-01-02T15:04:05Z")
	}

	return map[string]interface{}{
		"tool_result":    fmt.Sprintf("Operation %s is %s (%s) for %s", operationID, status, string(out.Type), domainName),
		"status":         status,
		"operation_type": string(out.Type),
		"message":        aws.ToString(out.Message),
		"domain_name":    domainName,
		"submitted_date": submitted,
	}, nil
}
