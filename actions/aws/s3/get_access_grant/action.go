// Package aws_s3_get_access_grant retrieves an S3 Access Grant by ID.
package aws_s3_get_access_grant

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
	Name         = "AWS S3 Get Access Grant"
	Description  = "Retrieve the details of an S3 Access Grant by its ID."
	Website      = "https://www.flomation.co"
	Icon         = "key+magnifying-glass"
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
	{Name: "access_grant_id", Type: core.ConnectionTypeString, Label: "Access Grant ID", Placeholder: "The ID returned when the grant was created", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "access_grant_id", Type: core.ConnectionTypeString, Label: "Access Grant ID"},
	{Name: "permission", Type: core.ConnectionTypeString, Label: "Permission"},
	{Name: "grant_scope", Type: core.ConnectionTypeString, Label: "Grant Scope"},
	{Name: "grantee", Type: core.ConnectionTypeString, Label: "Grantee (JSON)"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	grantID := strings.TrimSpace(awscommon.InputString("access_grant_id", inputs))
	if grantID == "" {
		return nil, fmt.Errorf("access_grant_id is required")
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

	out, err := client.GetAccessGrant(ctx, &s3control.GetAccessGrantInput{
		AccountId:     aws.String(accountID),
		AccessGrantId: aws.String(grantID),
	})
	if err != nil {
		return nil, err
	}

	permission := string(out.Permission)
	grantScope := aws.ToString(out.GrantScope)

	granteeJSON := ""
	if out.Grantee != nil {
		g := map[string]interface{}{
			"grantee_type":       string(out.Grantee.GranteeType),
			"grantee_identifier": aws.ToString(out.Grantee.GranteeIdentifier),
		}
		if b, err := json.Marshal(g); err == nil {
			granteeJSON = string(b)
		}
	}

	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Access grant %s: %s on %s", grantID, permission, grantScope),
		"access_grant_id": aws.ToString(out.AccessGrantId),
		"permission":      permission,
		"grant_scope":     grantScope,
		"grantee":         granteeJSON,
	}, nil
}
