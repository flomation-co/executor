// Package aws_s3_get_access_point_for_object_lambda retrieves an S3 Object Lambda Access Point.
package aws_s3_get_access_point_for_object_lambda

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
	Name         = "AWS S3 Get Object Lambda Access Point"
	Description  = "Retrieve details of an S3 Object Lambda Access Point by name."
	Website      = "https://www.flomation.co"
	Icon         = "code+magnifying-glass"
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
	{Name: "name", Type: core.ConnectionTypeString, Label: "Object Lambda Access Point Name", Placeholder: "my-olap", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name"},
	{Name: "alias", Type: core.ConnectionTypeString, Label: "Alias (JSON)"},
	{Name: "public_access_block", Type: core.ConnectionTypeString, Label: "Public Access Block (JSON)"},
	{Name: "creation_date", Type: core.ConnectionTypeString, Label: "Creation Date"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	name := strings.TrimSpace(awscommon.InputString("name", inputs))
	if name == "" {
		return nil, fmt.Errorf("name is required")
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

	out, err := client.GetAccessPointForObjectLambda(ctx, &s3control.GetAccessPointForObjectLambdaInput{
		AccountId: aws.String(accountID),
		Name:      aws.String(name),
	})
	if err != nil {
		return nil, err
	}

	var aliasJSON string
	if out.Alias != nil {
		if b, mErr := json.Marshal(out.Alias); mErr == nil {
			aliasJSON = string(b)
		}
	}
	var pabJSON string
	if out.PublicAccessBlockConfiguration != nil {
		if b, mErr := json.Marshal(out.PublicAccessBlockConfiguration); mErr == nil {
			pabJSON = string(b)
		}
	}
	var creationDate string
	if out.CreationDate != nil {
		creationDate = out.CreationDate.Format("2006-01-02T15:04:05Z07:00")
	}

	return map[string]interface{}{
		"tool_result":         fmt.Sprintf("Retrieved Object Lambda Access Point %s", aws.ToString(out.Name)),
		"name":                aws.ToString(out.Name),
		"alias":               aliasJSON,
		"public_access_block": pabJSON,
		"creation_date":       creationDate,
	}, nil
}
