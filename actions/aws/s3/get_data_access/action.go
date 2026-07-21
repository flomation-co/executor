// Package aws_s3_get_data_access vends temporary scoped credentials for an S3
// target via S3 Access Grants.
package aws_s3_get_data_access

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
	Name         = "AWS S3 Get Data Access"
	Description  = "Vend temporary scoped credentials for an S3 target via S3 Access Grants."
	Website      = "https://www.flomation.co"
	Icon         = "key+shield-halved"
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
	{Name: "target", Type: core.ConnectionTypeString, Label: "Target S3 URI", Placeholder: "s3://my-bucket/prefix/", Required: true},
	{Name: "permission", Type: core.ConnectionTypeString, Label: "Permission", Required: true, Options: []core.ConnectionOption{
		{Name: "Read", Value: "READ"},
		{Name: "Write", Value: "WRITE"},
		{Name: "Read & Write", Value: "READWRITE"},
	}},
	{Name: "duration_seconds", Type: core.ConnectionTypeInteger, Label: "Duration (seconds, optional)", Placeholder: "900-43200; defaults to 3600 (1 hour)"},
	{Name: "target_type", Type: core.ConnectionTypeString, Label: "Target Type (optional)", Placeholder: "Set to Object when the target is a single object", Options: []core.ConnectionOption{
		{Name: "Object", Value: "Object"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "access_key_id", Type: core.ConnectionTypeString, Label: "Temporary Access Key ID (sensitive)"},
	{Name: "secret_access_key", Type: core.ConnectionTypeString, Label: "Temporary Secret Access Key (sensitive)"},
	{Name: "session_token", Type: core.ConnectionTypeString, Label: "Temporary Session Token (sensitive)"},
	{Name: "expiration", Type: core.ConnectionTypeString, Label: "Credentials Expiration"},
	{Name: "matched_grant_target", Type: core.ConnectionTypeString, Label: "Matched Grant Target"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	target := strings.TrimSpace(awscommon.InputString("target", inputs))
	if target == "" {
		return nil, fmt.Errorf("target is required")
	}
	permission := strings.TrimSpace(awscommon.InputString("permission", inputs))
	if permission == "" {
		return nil, fmt.Errorf("permission is required")
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

	in := &s3control.GetDataAccessInput{
		AccountId:  aws.String(accountID),
		Target:     aws.String(target),
		Permission: s3ctltypes.Permission(permission),
	}
	if seconds, ok := awscommon.InputInt("duration_seconds", inputs); ok {
		in.DurationSeconds = aws.Int32(int32(seconds))
	}
	if targetType := strings.TrimSpace(awscommon.InputString("target_type", inputs)); targetType != "" {
		in.TargetType = s3ctltypes.S3PrefixType(targetType)
	}

	out, err := client.GetDataAccess(ctx, in)
	if err != nil {
		return nil, err
	}

	var accessKeyID, secretAccessKey, sessionToken, expiration string
	if out.Credentials != nil {
		accessKeyID = aws.ToString(out.Credentials.AccessKeyId)
		secretAccessKey = aws.ToString(out.Credentials.SecretAccessKey)
		sessionToken = aws.ToString(out.Credentials.SessionToken)
		if out.Credentials.Expiration != nil {
			expiration = out.Credentials.Expiration.Format("2006-01-02T15:04:05Z07:00")
		}
	}
	matchedTarget := aws.ToString(out.MatchedGrantTarget)

	return map[string]interface{}{
		"tool_result":          fmt.Sprintf("Vended temporary %s credentials for %s (expire %s)", permission, target, expiration),
		"access_key_id":        accessKeyID,
		"secret_access_key":    secretAccessKey,
		"session_token":        sessionToken,
		"expiration":           expiration,
		"matched_grant_target": matchedTarget,
	}, nil
}
