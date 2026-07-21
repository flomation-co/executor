// Package aws_rds_describe_pending_maintenance_actions lists pending maintenance
// actions for RDS resources, optionally narrowed to a single resource ARN.
package aws_rds_describe_pending_maintenance_actions

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
	Name         = "AWS RDS Describe Pending Maintenance Actions"
	Description  = "List pending maintenance actions for RDS resources, optionally by resource ARN."
	Website      = "https://www.flomation.co"
	Icon         = "calendar+magnifying-glass"
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
	{Name: "resource_identifier", Type: core.ConnectionTypeString, Label: "Resource ARN (optional)", Placeholder: "Leave blank to list all; e.g. arn:aws:rds:...:db:my-database"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "pending", Type: core.ConnectionTypeObject, Label: "Pending Maintenance Actions"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.DescribePendingMaintenanceActionsInput{}
	if res := awscommon.InputString("resource_identifier", inputs); res != "" {
		in.ResourceIdentifier = aws.String(res)
	}

	var pending []map[string]interface{}
	paginator := rds.NewDescribePendingMaintenanceActionsPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for i := range page.PendingMaintenanceActions {
			pending = append(pending, flattenResourceActions(page.PendingMaintenanceActions[i]))
		}
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found pending maintenance for %d resource(s)", len(pending)),
		"pending":     pending,
		"count":       len(pending),
	}, nil
}

func flattenResourceActions(r rdstypes.ResourcePendingMaintenanceActions) map[string]interface{} {
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
