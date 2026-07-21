// Package aws_s3_create_access_grant creates an S3 Access Grant.
package aws_s3_create_access_grant

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
	Name         = "AWS S3 Create Access Grant"
	Description  = "Grant an IAM or directory identity scoped access to S3 data via S3 Access Grants."
	Website      = "https://www.flomation.co"
	Icon         = "key+plus"
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
	{Name: "access_grants_location_id", Type: core.ConnectionTypeString, Label: "Access Grants Location ID", Placeholder: "default, or the registered location ID", Required: true},
	{Name: "permission", Type: core.ConnectionTypeString, Label: "Permission", Required: true, Options: []core.ConnectionOption{
		{Name: "Read", Value: "READ"},
		{Name: "Write", Value: "WRITE"},
		{Name: "Read & Write", Value: "READWRITE"},
	}},
	{Name: "grantee_type", Type: core.ConnectionTypeString, Label: "Grantee Type", Required: true, Options: []core.ConnectionOption{
		{Name: "IAM User or Role", Value: "IAM"},
		{Name: "Directory User", Value: "DIRECTORY_USER"},
		{Name: "Directory Group", Value: "DIRECTORY_GROUP"},
	}},
	{Name: "grantee_identifier", Type: core.ConnectionTypeString, Label: "Grantee Identifier", Placeholder: "IAM ARN, or the directory user/group UUID", Required: true},
	{Name: "s3_prefix_type", Type: core.ConnectionTypeString, Label: "S3 Prefix Type (optional)", Placeholder: "Set to Object when the grant scope is a single object", Options: []core.ConnectionOption{
		{Name: "Object", Value: "Object"},
	}},
	{Name: "access_grants_location_configuration", Type: core.ConnectionTypeString, Label: "Location Configuration (JSON, optional)", Placeholder: `{"S3SubPrefix":"reports/*"}`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "access_grant_id", Type: core.ConnectionTypeString, Label: "Access Grant ID"},
	{Name: "access_grant_arn", Type: core.ConnectionTypeString, Label: "Access Grant ARN"},
	{Name: "grant_scope", Type: core.ConnectionTypeString, Label: "Grant Scope"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	locationID := strings.TrimSpace(awscommon.InputString("access_grants_location_id", inputs))
	if locationID == "" {
		return nil, fmt.Errorf("access_grants_location_id is required")
	}
	permission := strings.TrimSpace(awscommon.InputString("permission", inputs))
	if permission == "" {
		return nil, fmt.Errorf("permission is required")
	}
	granteeType := strings.TrimSpace(awscommon.InputString("grantee_type", inputs))
	if granteeType == "" {
		return nil, fmt.Errorf("grantee_type is required")
	}
	granteeIdentifier := strings.TrimSpace(awscommon.InputString("grantee_identifier", inputs))
	if granteeIdentifier == "" {
		return nil, fmt.Errorf("grantee_identifier is required")
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

	in := &s3control.CreateAccessGrantInput{
		AccountId:              aws.String(accountID),
		AccessGrantsLocationId: aws.String(locationID),
		Permission:             s3ctltypes.Permission(permission),
		Grantee: &s3ctltypes.Grantee{
			GranteeType:       s3ctltypes.GranteeType(granteeType),
			GranteeIdentifier: aws.String(granteeIdentifier),
		},
	}
	if prefixType := strings.TrimSpace(awscommon.InputString("s3_prefix_type", inputs)); prefixType != "" {
		in.S3PrefixType = s3ctltypes.S3PrefixType(prefixType)
	}
	if locCfg := strings.TrimSpace(awscommon.InputString("access_grants_location_configuration", inputs)); locCfg != "" {
		var cfgLoc s3ctltypes.AccessGrantsLocationConfiguration
		if err := json.Unmarshal([]byte(locCfg), &cfgLoc); err != nil {
			return nil, fmt.Errorf("access_grants_location_configuration is not valid JSON: %w", err)
		}
		in.AccessGrantsLocationConfiguration = &cfgLoc
	}

	out, err := client.CreateAccessGrant(ctx, in)
	if err != nil {
		return nil, err
	}

	grantID := aws.ToString(out.AccessGrantId)
	grantArn := aws.ToString(out.AccessGrantArn)
	grantScope := aws.ToString(out.GrantScope)
	return map[string]interface{}{
		"tool_result":      fmt.Sprintf("Created access grant %s granting %s to %s", grantID, permission, granteeIdentifier),
		"access_grant_id":  grantID,
		"access_grant_arn": grantArn,
		"grant_scope":      grantScope,
	}, nil
}
