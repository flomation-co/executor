// Package aws_s3_create_access_point_for_object_lambda creates an S3 Object Lambda Access Point.
package aws_s3_create_access_point_for_object_lambda

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	s3control "github.com/aws/aws-sdk-go-v2/service/s3control"
	s3ctltypes "github.com/aws/aws-sdk-go-v2/service/s3control/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS S3 Create Object Lambda Access Point"
	Description  = "Create an S3 Object Lambda Access Point from a JSON configuration document."
	Website      = "https://www.flomation.co"
	Icon         = "code+plus"
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
	{Name: "configuration", Type: core.ConnectionTypeString, Label: "Configuration (JSON)", Placeholder: `{"SupportingAccessPoint":"arn:aws:s3:...:accesspoint/my-ap","TransformationConfigurations":[{"Actions":["GetObject"],"ContentTransformation":{"AwsLambda":{"FunctionArn":"arn:aws:lambda:..."}}}]}`, Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "object_lambda_access_point_arn", Type: core.ConnectionTypeString, Label: "Object Lambda Access Point ARN"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	name := strings.TrimSpace(awscommon.InputString("name", inputs))
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	configRaw := strings.TrimSpace(awscommon.InputString("configuration", inputs))
	if configRaw == "" {
		return nil, fmt.Errorf("configuration JSON is required")
	}
	var configuration s3ctltypes.ObjectLambdaConfiguration
	if err := json.Unmarshal([]byte(configRaw), &configuration); err != nil {
		return nil, fmt.Errorf("configuration must be a JSON object: %w", err)
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

	out, err := client.CreateAccessPointForObjectLambda(ctx, &s3control.CreateAccessPointForObjectLambdaInput{
		AccountId:     aws.String(accountID),
		Name:          aws.String(name),
		Configuration: &configuration,
	})
	if err != nil {
		return nil, err
	}

	arn := aws.ToString(out.ObjectLambdaAccessPointArn)
	return map[string]interface{}{
		"tool_result":                    fmt.Sprintf("Created Object Lambda Access Point %s", name),
		"object_lambda_access_point_arn": arn,
	}, nil
}
