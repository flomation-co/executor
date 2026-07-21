// Package aws_s3_list_access_grants lists S3 Access Grants.
package aws_s3_list_access_grants

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
	Name         = "AWS S3 List Access Grants"
	Description  = "List the S3 Access Grants in an instance, optionally filtered by grantee or permission."
	Website      = "https://www.flomation.co"
	Icon         = "key+list"
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
	{Name: "grantee_identifier", Type: core.ConnectionTypeString, Label: "Grantee Identifier (filter, optional)", Placeholder: "IAM ARN, or a directory user/group UUID"},
	{Name: "permission", Type: core.ConnectionTypeString, Label: "Permission (filter, optional)", Options: []core.ConnectionOption{
		{Name: "Read", Value: "READ"},
		{Name: "Write", Value: "WRITE"},
		{Name: "Read & Write", Value: "READWRITE"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "grants", Type: core.ConnectionTypeString, Label: "Grants (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Grant count"},
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

	in := &s3control.ListAccessGrantsInput{
		AccountId: aws.String(accountID),
	}
	if granteeID := strings.TrimSpace(awscommon.InputString("grantee_identifier", inputs)); granteeID != "" {
		in.GranteeIdentifier = aws.String(granteeID)
	}
	if permission := strings.TrimSpace(awscommon.InputString("permission", inputs)); permission != "" {
		in.Permission = s3ctltypes.Permission(permission)
	}

	out, err := client.ListAccessGrants(ctx, in)
	if err != nil {
		return nil, err
	}

	type grantEntry struct {
		AccessGrantID string `json:"access_grant_id"`
		Permission    string `json:"permission"`
		GrantScope    string `json:"grant_scope"`
	}
	entries := make([]grantEntry, 0, len(out.AccessGrantsList))
	for _, g := range out.AccessGrantsList {
		entries = append(entries, grantEntry{
			AccessGrantID: aws.ToString(g.AccessGrantId),
			Permission:    string(g.Permission),
			GrantScope:    aws.ToString(g.GrantScope),
		})
	}

	grantsJSON := "[]"
	if b, err := json.Marshal(entries); err == nil {
		grantsJSON = string(b)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d access grant(s)", len(entries)),
		"grants":      grantsJSON,
		"count":       len(entries),
	}, nil
}
