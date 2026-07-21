// Package aws_s3_list_access_grants_locations lists registered S3 Access Grants locations.
package aws_s3_list_access_grants_locations

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
	Name         = "AWS S3 List Access Grants Locations"
	Description  = "List locations registered with an S3 Access Grants instance."
	Website      = "https://www.flomation.co"
	Icon         = "map+list"
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
	{Name: "location_scope", Type: core.ConnectionTypeString, Label: "Location Scope (optional filter)", Placeholder: "s3://my-bucket/prefix/"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "locations", Type: core.ConnectionTypeString, Label: "Locations (JSON)"},
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

	in := &s3control.ListAccessGrantsLocationsInput{
		AccountId: aws.String(accountID),
	}
	if scope := awscommon.InputString("location_scope", inputs); scope != "" {
		in.LocationScope = aws.String(scope)
	}

	out, err := client.ListAccessGrantsLocations(ctx, in)
	if err != nil {
		return nil, err
	}

	type entry struct {
		ID            string `json:"access_grants_location_id"`
		ARN           string `json:"access_grants_location_arn"`
		LocationScope string `json:"location_scope,omitempty"`
		IAMRoleARN    string `json:"iam_role_arn,omitempty"`
		CreatedAt     string `json:"created_at,omitempty"`
	}
	locations := make([]entry, 0, len(out.AccessGrantsLocationsList))
	for _, l := range out.AccessGrantsLocationsList {
		e := entry{
			ID:            aws.ToString(l.AccessGrantsLocationId),
			ARN:           aws.ToString(l.AccessGrantsLocationArn),
			LocationScope: aws.ToString(l.LocationScope),
			IAMRoleARN:    aws.ToString(l.IAMRoleArn),
		}
		if l.CreatedAt != nil {
			e.CreatedAt = l.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
		}
		locations = append(locations, e)
	}

	data, err := json.Marshal(locations)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d S3 Access Grants location(s)", len(locations)),
		"locations":   string(data),
		"count":       len(locations),
	}, nil
}
