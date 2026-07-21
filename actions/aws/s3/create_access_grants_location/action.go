// Package aws_s3_create_access_grants_location registers an S3 Access Grants location.
package aws_s3_create_access_grants_location

import (
	"context"
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
	Name         = "AWS S3 Create Access Grants Location"
	Description  = "Register an S3 location with Access Grants, backed by an IAM role."
	Website      = "https://www.flomation.co"
	Icon         = "map+plus"
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
	{Name: "location_scope", Type: core.ConnectionTypeString, Label: "Location Scope", Placeholder: "s3://my-bucket/prefix/", Required: true},
	{Name: "iam_role_arn", Type: core.ConnectionTypeString, Label: "IAM Role ARN", Placeholder: "arn:aws:iam::<account>:role/AccessGrantsLocationRole", Required: true},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "access_grants_location_id", Type: core.ConnectionTypeString, Label: "Access Grants Location ID"},
	{Name: "access_grants_location_arn", Type: core.ConnectionTypeString, Label: "Access Grants Location ARN"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	locationScope := awscommon.InputString("location_scope", inputs)
	if locationScope == "" {
		return nil, fmt.Errorf("location_scope is required")
	}
	iamRoleARN := awscommon.InputString("iam_role_arn", inputs)
	if iamRoleARN == "" {
		return nil, fmt.Errorf("iam_role_arn is required")
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

	in := &s3control.CreateAccessGrantsLocationInput{
		AccountId:     aws.String(accountID),
		LocationScope: aws.String(locationScope),
		IAMRoleArn:    aws.String(iamRoleARN),
	}
	if conn := core.FindConnection("tags", inputs); conn != nil {
		var tags []s3ctltypes.Tag
		for _, kv := range conn.KeyValuePairs() {
			k := strings.TrimSpace(kv.Key)
			if k == "" {
				continue
			}
			tags = append(tags, s3ctltypes.Tag{
				Key:   aws.String(k),
				Value: aws.String(kv.Value),
			})
		}
		if len(tags) > 0 {
			in.Tags = tags
		}
	}

	out, err := client.CreateAccessGrantsLocation(ctx, in)
	if err != nil {
		return nil, err
	}

	id := aws.ToString(out.AccessGrantsLocationId)
	arn := aws.ToString(out.AccessGrantsLocationArn)
	return map[string]interface{}{
		"tool_result":                fmt.Sprintf("Registered S3 Access Grants location %s (%s)", id, locationScope),
		"access_grants_location_id":  id,
		"access_grants_location_arn": arn,
	}, nil
}
