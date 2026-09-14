// Package adgroup_status_set enables, pauses or removes a Google Ads ad group.
package adgroup_status_set

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	gads "flomation.app/automate/executor/actions/marketing/google_ads"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Ad Group: Set Status"
	Description  = "Enable, pause or remove a Google Ads ad group. Removing is permanent."
	Summary      = "Pause or enable a Google Ads ad group"
	Website      = "https://www.flomation.co"
	Icon         = "googleads+pause"
	Date         = "12/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google Ads Connection", Placeholder: "${credentials.MyGoogleAds}", Required: true},
	{Name: "customer_id", Type: core.ConnectionTypeString, Label: "Customer ID (the ad account, e.g. 123-456-7890)", Required: true},
	{Name: "login_customer_id", Type: core.ConnectionTypeString, Label: "Manager Account ID (only when acting through an MCC)", Placeholder: "${credentials.MyGoogleAds.login_customer_id}"},
	{Name: "ad_group_id", Type: core.ConnectionTypeString, Label: "Ad Group ID", Required: true},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status", Required: true, Options: []core.ConnectionOption{{Name: "Pause", Value: "PAUSED"}, {Name: "Enable", Value: "ENABLED"}, {Name: "Remove (permanent)", Value: "REMOVED"}}},
	{Name: "validate_only", Type: core.ConnectionTypeBoolean, Label: "Dry run — validate without changing anything"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Ad Group ID"},
	{Name: "resource_name", Type: core.ConnectionTypeString, Label: "Ad Group Resource Name"},
	{Name: "resource_names", Type: core.ConnectionTypeObject, Label: "Changed Resource Names"},
	{Name: "dry_run", Type: core.ConnectionTypeBoolean, Label: "Was a Dry Run"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, customerID, login, err := gads.GetAuth(inputs)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}
	adGroupID, err := gads.RequiredString("ad_group_id", inputs)
	if err != nil {
		return gads.ErrorResult("an ad group ID is required"), nil
	}

	status := strings.ToUpper(gads.OptionalString("status", inputs))
	switch status {
	case "ENABLED", "PAUSED", "REMOVED":
	case "":
		return gads.ErrorResult("a status is required (Enable, Pause or Remove)"), nil
	default:
		if status == "ACTIVE" {
			return gads.ErrorResult("Google Ads uses ENABLED rather than ACTIVE — set the status to Enable"), nil
		}
		return gads.ErrorResult(fmt.Sprintf("%q is not an ad group status; use ENABLED, PAUSED or REMOVED", status)), nil
	}

	operations := []interface{}{map[string]interface{}{
		"update": map[string]interface{}{
			"resourceName": fmt.Sprintf("customers/%s/adGroups/%s", customerID, adGroupID),
			"status":       status,
		},
		"updateMask": "status",
	}}
	opt := gads.MutateOptions{ValidateOnly: gads.OptionalBool("validate_only", inputs), PartialFailure: false}

	resp, err := gads.NewClient(token, login).Mutate(flow, customerID, "adGroups", operations, opt)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf("Set ad group %s to %s", adGroupID, status)
	if status == "REMOVED" {
		summary += " (removal is permanent and cannot be undone)"
	}
	return gads.MutateResult(resp, opt, summary, nil), nil
}
