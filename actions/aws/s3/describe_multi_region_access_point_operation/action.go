// Package aws_s3_describe_multi_region_access_point_operation describes an
// asynchronous S3 Multi-Region Access Point operation.
package aws_s3_describe_multi_region_access_point_operation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	s3control "github.com/aws/aws-sdk-go-v2/service/s3control"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS S3 Describe Multi-Region Access Point Operation"
	Description  = "Query the status of an async S3 Multi-Region Access Point operation by request token."
	Website      = "https://www.flomation.co"
	Icon         = "globe+magnifying-glass"
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
	{Name: "aws_region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "us-west-2 (MRAP control lives in us-west-2)", Required: true},
	{Name: "aws_session_token", Type: core.ConnectionTypeSecret, Label: "Session Token (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "assume_role_arn", Type: core.ConnectionTypeString, Label: "Role ARN to Assume", Placeholder: "arn:aws:iam::<your-account>:role/FlomationAccess", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"assume_role"}}},
	{Name: "external_id", Type: core.ConnectionTypeString, Label: "Assume Role External ID (optional)", Placeholder: "Must match the External ID in the role's trust policy", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"assume_role"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "AWS Role Credential", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"credential"}}},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "AWS Account ID", Placeholder: "12-digit account ID; leave blank to auto-detect from the credential"},
	{Name: "request_token_arn", Type: core.ConnectionTypeString, Label: "Request Token ARN", Placeholder: "The token returned by Create/Delete Multi-Region Access Point", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Request Status"},
	{Name: "async_operation", Type: core.ConnectionTypeString, Label: "Async Operation (JSON)"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	requestToken := strings.TrimSpace(awscommon.InputString("request_token_arn", inputs))
	if requestToken == "" {
		return nil, fmt.Errorf("request_token_arn is required")
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

	out, err := client.DescribeMultiRegionAccessPointOperation(ctx, &s3control.DescribeMultiRegionAccessPointOperationInput{
		AccountId:       aws.String(accountID),
		RequestTokenARN: aws.String(requestToken),
	})
	if err != nil {
		return nil, err
	}

	var status, asyncJSON string
	if out.AsyncOperation != nil {
		status = aws.ToString(out.AsyncOperation.RequestStatus)
		detail := map[string]interface{}{
			"operation":         string(out.AsyncOperation.Operation),
			"request_status":    status,
			"request_token_arn": aws.ToString(out.AsyncOperation.RequestTokenARN),
		}
		if out.AsyncOperation.CreationTime != nil {
			detail["creation_time"] = out.AsyncOperation.CreationTime.Format("2006-01-02T15:04:05Z07:00")
		}
		if b, err := json.Marshal(detail); err == nil {
			asyncJSON = string(b)
		}
	}

	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Multi-Region Access Point operation status: %s", status),
		"status":          status,
		"async_operation": asyncJSON,
	}, nil
}
