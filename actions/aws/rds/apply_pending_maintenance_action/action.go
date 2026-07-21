// Package aws_rds_apply_pending_maintenance_action applies (or opts in/out of) a
// pending maintenance action on an RDS resource.
package aws_rds_apply_pending_maintenance_action

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Apply Pending Maintenance Action"
	Description  = "Apply or opt in/out of a pending maintenance action on an RDS resource."
	Website      = "https://www.flomation.co"
	Icon         = "calendar+play"
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
	{Name: "resource_identifier", Type: core.ConnectionTypeString, Label: "Resource ARN", Placeholder: "arn:aws:rds:...:db:my-database", Required: true},
	{Name: "apply_action", Type: core.ConnectionTypeString, Label: "Maintenance Action", Placeholder: "system-update", Required: true},
	{Name: "opt_in_type", Type: core.ConnectionTypeString, Label: "Opt-in Type", Required: true, Options: []core.ConnectionOption{
		{Name: "Immediate", Value: "immediate"},
		{Name: "Next Maintenance Window", Value: "next-maintenance"},
		{Name: "Undo Opt-in", Value: "undo-opt-in"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "resource", Type: core.ConnectionTypeObject, Label: "Resource Pending Actions"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	res := awscommon.InputString("resource_identifier", inputs)
	action := awscommon.InputString("apply_action", inputs)
	optIn := awscommon.InputString("opt_in_type", inputs)
	if res == "" || action == "" || optIn == "" {
		return nil, fmt.Errorf("resource identifier, maintenance action and opt-in type are all required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	out, err := client.ApplyPendingMaintenanceAction(ctx, &rds.ApplyPendingMaintenanceActionInput{
		ResourceIdentifier: aws.String(res),
		ApplyAction:        aws.String(action),
		OptInType:          aws.String(optIn),
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Applied maintenance action %q (%s) to %q", action, optIn, res),
		"resource":    flattenResourceActions(out.ResourcePendingMaintenanceActions),
	}, nil
}

func flattenResourceActions(r *rdstypes.ResourcePendingMaintenanceActions) map[string]interface{} {
	if r == nil {
		return nil
	}
	var actions []map[string]interface{}
	for _, a := range r.PendingMaintenanceActionDetails {
		var currentApplyDate string
		if a.CurrentApplyDate != nil {
			currentApplyDate = a.CurrentApplyDate.Format("2006-01-02T15:04:05Z07:00")
		}
		actions = append(actions, map[string]interface{}{
			"action":             aws.ToString(a.Action),
			"opt_in_status":      aws.ToString(a.OptInStatus),
			"current_apply_date": currentApplyDate,
			"description":        aws.ToString(a.Description),
		})
	}
	return map[string]interface{}{
		"resource_identifier": aws.ToString(r.ResourceIdentifier),
		"actions":             actions,
	}
}
