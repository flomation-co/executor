// Package aws_vpc_delete_managed_prefix_list deletes a customer-managed prefix list.
package aws_vpc_delete_managed_prefix_list

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS VPC Delete Managed Prefix List"
	Description  = "Delete a customer-managed prefix list."
	Website      = "https://www.flomation.co"
	Icon         = "list+trash"
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
	{Name: "prefix_list_id", Type: core.ConnectionTypeString, Label: "Prefix List ID", Placeholder: "pl-0abc...", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "prefix_list", Type: core.ConnectionTypeObject, Label: "Prefix List"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	plID := strings.TrimSpace(awscommon.InputString("prefix_list_id", inputs))
	if plID == "" {
		return nil, fmt.Errorf("prefix_list_id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	out, err := client.DeleteManagedPrefixList(ctx, &ec2.DeleteManagedPrefixListInput{
		PrefixListId: aws.String(plID),
	})
	if err != nil {
		return nil, err
	}

	pl := map[string]interface{}{}
	if out.PrefixList != nil {
		pl = map[string]interface{}{
			"prefix_list_id":   aws.ToString(out.PrefixList.PrefixListId),
			"prefix_list_name": aws.ToString(out.PrefixList.PrefixListName),
			"state":            string(out.PrefixList.State),
		}
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Deleting managed prefix list %s", plID),
		"prefix_list": pl,
	}, nil
}
