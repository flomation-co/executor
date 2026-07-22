// Package aws_iam_list_instance_profiles lists IAM instance profiles.
package aws_iam_list_instance_profiles

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS IAM List Instance Profiles"
	Description  = "List IAM instance profiles, optionally filtered by path prefix."
	Website      = "https://www.flomation.co"
	Icon         = "id-badge+list"
	Date         = "22/07/2026"
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
	{Name: "path_prefix", Type: core.ConnectionTypeString, Label: "Path Prefix (optional)", Placeholder: "/division_abc/"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "instance_profiles", Type: core.ConnectionTypeString, Label: "Instance Profiles (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

type profileSummary struct {
	Name string `json:"name"`
	Arn  string `json:"arn"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := iam.NewFromConfig(cfg)

	in := &iam.ListInstanceProfilesInput{}
	if p := strings.TrimSpace(awscommon.InputString("path_prefix", inputs)); p != "" {
		in.PathPrefix = aws.String(p)
	}

	profiles := []profileSummary{}
	paginator := iam.NewListInstanceProfilesPaginator(client, in)
	for paginator.HasMorePages() {
		page, perr := paginator.NextPage(ctx)
		if perr != nil {
			return nil, perr
		}
		for _, p := range page.InstanceProfiles {
			profiles = append(profiles, profileSummary{
				Name: aws.ToString(p.InstanceProfileName),
				Arn:  aws.ToString(p.Arn),
			})
		}
	}

	profilesJSON, err := json.Marshal(profiles)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result":       fmt.Sprintf("Found %d instance profiles", len(profiles)),
		"instance_profiles": string(profilesJSON),
		"count":             len(profiles),
	}, nil
}
