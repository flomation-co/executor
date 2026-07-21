// Package aws_s3_create_access_point creates an AWS S3 access point for a bucket.
package aws_s3_create_access_point

import (
	"context"
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	s3control "github.com/aws/aws-sdk-go-v2/service/s3control"
	s3ctltypes "github.com/aws/aws-sdk-go-v2/service/s3control/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS S3 Create Access Point"
	Description  = "Create an S3 access point for a bucket, optionally VPC-scoped."
	Website      = "https://www.flomation.co"
	Icon         = "circle-nodes+plus"
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
	{Name: "name", Type: core.ConnectionTypeString, Label: "Access Point Name", Placeholder: "my-access-point", Required: true},
	{Name: "bucket", Type: core.ConnectionTypeString, Label: "Bucket Name", Placeholder: "my-bucket", Required: true},
	{Name: "vpc_id", Type: core.ConnectionTypeString, Label: "VPC ID (optional)", Placeholder: "vpc-1a2b3c4d — makes the access point VPC-only"},
	{Name: "public_access_block", Type: core.ConnectionTypeString, Label: "Public Access Block (JSON, optional)", Placeholder: `{"BlockPublicAcls":true,"IgnorePublicAcls":true,"BlockPublicPolicy":true,"RestrictPublicBuckets":true}`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "access_point_arn", Type: core.ConnectionTypeString, Label: "Access Point ARN"},
	{Name: "alias", Type: core.ConnectionTypeString, Label: "Alias"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	name := awscommon.InputString("name", inputs)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	bucket := awscommon.InputString("bucket", inputs)
	if bucket == "" {
		return nil, fmt.Errorf("bucket is required")
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

	in := &s3control.CreateAccessPointInput{
		AccountId: aws.String(accountID),
		Name:      aws.String(name),
		Bucket:    aws.String(bucket),
	}
	if vpcID := awscommon.InputString("vpc_id", inputs); vpcID != "" {
		in.VpcConfiguration = &s3ctltypes.VpcConfiguration{VpcId: aws.String(vpcID)}
	}
	if pab := awscommon.InputString("public_access_block", inputs); pab != "" {
		var cfgPAB s3ctltypes.PublicAccessBlockConfiguration
		if err := json.Unmarshal([]byte(pab), &cfgPAB); err != nil {
			return nil, fmt.Errorf("public_access_block is not valid JSON: %w", err)
		}
		in.PublicAccessBlockConfiguration = &cfgPAB
	}

	out, err := client.CreateAccessPoint(ctx, in)
	if err != nil {
		return nil, err
	}

	arn := aws.ToString(out.AccessPointArn)
	alias := aws.ToString(out.Alias)
	return map[string]interface{}{
		"tool_result":      fmt.Sprintf("Created access point %s on bucket %s", name, bucket),
		"access_point_arn": arn,
		"alias":            alias,
	}, nil
}
