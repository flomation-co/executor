// Package aws_s3_create_multi_region_access_point creates an S3 Multi-Region Access Point (async).
package aws_s3_create_multi_region_access_point

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
	Name         = "AWS S3 Create Multi-Region Access Point"
	Description  = "Create an S3 Multi-Region Access Point (async; control plane is us-west-2)."
	Website      = "https://www.flomation.co"
	Icon         = "globe+plus"
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
	{Name: "name", Type: core.ConnectionTypeString, Label: "Access Point Name", Required: true},
	{Name: "regions", Type: core.ConnectionTypeString, Label: "Buckets", Placeholder: "Comma-separated bucket names, one per Region", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "request_token_arn", Type: core.ConnectionTypeString, Label: "Request Token ARN"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	name := strings.TrimSpace(awscommon.InputString("name", inputs))
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	var regions []s3ctltypes.Region
	for _, b := range strings.Split(awscommon.InputString("regions", inputs), ",") {
		b = strings.TrimSpace(b)
		if b == "" {
			continue
		}
		regions = append(regions, s3ctltypes.Region{Bucket: aws.String(b)})
	}
	if len(regions) == 0 {
		return nil, fmt.Errorf("at least one bucket is required")
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

	out, err := client.CreateMultiRegionAccessPoint(ctx, &s3control.CreateMultiRegionAccessPointInput{
		AccountId:   aws.String(accountID),
		ClientToken: aws.String("flomation-mrap-create-" + name),
		Details: &s3ctltypes.CreateMultiRegionAccessPointInput{
			Name:    aws.String(name),
			Regions: regions,
		},
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result":       fmt.Sprintf("Requested Multi-Region Access Point %s (async; token %s)", name, aws.ToString(out.RequestTokenARN)),
		"request_token_arn": aws.ToString(out.RequestTokenARN),
	}, nil
}
