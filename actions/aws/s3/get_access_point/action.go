// Package aws_s3_get_access_point reads the configuration of an AWS S3 access point.
package aws_s3_get_access_point

import (
	"context"
	"fmt"
	"time"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	s3control "github.com/aws/aws-sdk-go-v2/service/s3control"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS S3 Get Access Point"
	Description  = "Read the configuration of an AWS S3 access point."
	Website      = "https://www.flomation.co"
	Icon         = "circle-nodes+magnifying-glass"
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Access Point Name"},
	{Name: "bucket", Type: core.ConnectionTypeString, Label: "Bucket"},
	{Name: "network_origin", Type: core.ConnectionTypeString, Label: "Network Origin"},
	{Name: "vpc_id", Type: core.ConnectionTypeString, Label: "VPC ID"},
	{Name: "alias", Type: core.ConnectionTypeString, Label: "Alias"},
	{Name: "access_point_arn", Type: core.ConnectionTypeString, Label: "Access Point ARN"},
	{Name: "creation_date", Type: core.ConnectionTypeString, Label: "Creation Date"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	name := awscommon.InputString("name", inputs)
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

	out, err := client.GetAccessPoint(ctx, &s3control.GetAccessPointInput{
		AccountId: aws.String(accountID),
		Name:      aws.String(name),
	})
	if err != nil {
		return nil, err
	}

	bucket := aws.ToString(out.Bucket)
	networkOrigin := string(out.NetworkOrigin)
	var vpcID string
	if out.VpcConfiguration != nil {
		vpcID = aws.ToString(out.VpcConfiguration.VpcId)
	}
	alias := aws.ToString(out.Alias)
	arn := aws.ToString(out.AccessPointArn)
	var creationDate string
	if out.CreationDate != nil {
		creationDate = out.CreationDate.Format(time.RFC3339)
	}

	return map[string]interface{}{
		"tool_result":      fmt.Sprintf("Access point %s on bucket %s (%s)", name, bucket, networkOrigin),
		"name":             aws.ToString(out.Name),
		"bucket":           bucket,
		"network_origin":   networkOrigin,
		"vpc_id":           vpcID,
		"alias":            alias,
		"access_point_arn": arn,
		"creation_date":    creationDate,
	}, nil
}
