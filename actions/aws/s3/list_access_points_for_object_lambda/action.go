// Package aws_s3_list_access_points_for_object_lambda lists S3 Object Lambda Access Points.
package aws_s3_list_access_points_for_object_lambda

import (
	"context"
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	s3control "github.com/aws/aws-sdk-go-v2/service/s3control"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS S3 List Object Lambda Access Points"
	Description  = "List all S3 Object Lambda Access Points for an account."
	Website      = "https://www.flomation.co"
	Icon         = "code+list"
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "access_points", Type: core.ConnectionTypeString, Label: "Access Points (JSON)"},
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

	type accessPoint struct {
		Name                       string `json:"name"`
		ObjectLambdaAccessPointArn string `json:"object_lambda_access_point_arn"`
		Alias                      string `json:"alias"`
	}
	var points []accessPoint

	paginator := s3control.NewListAccessPointsForObjectLambdaPaginator(client, &s3control.ListAccessPointsForObjectLambdaInput{
		AccountId: aws.String(accountID),
	})
	for paginator.HasMorePages() {
		page, pErr := paginator.NextPage(ctx)
		if pErr != nil {
			return nil, pErr
		}
		for _, ap := range page.ObjectLambdaAccessPointList {
			var alias string
			if ap.Alias != nil {
				alias = aws.ToString(ap.Alias.Value)
			}
			points = append(points, accessPoint{
				Name:                       aws.ToString(ap.Name),
				ObjectLambdaAccessPointArn: aws.ToString(ap.ObjectLambdaAccessPointArn),
				Alias:                      alias,
			})
		}
	}

	b, err := json.Marshal(points)
	if err != nil {
		return nil, fmt.Errorf("marshal access points: %w", err)
	}

	return map[string]interface{}{
		"tool_result":   fmt.Sprintf("Found %d Object Lambda Access Point(s)", len(points)),
		"access_points": string(b),
		"count":         len(points),
	}, nil
}
