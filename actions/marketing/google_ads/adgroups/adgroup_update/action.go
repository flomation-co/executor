// Package adgroup_update changes settings on an existing Google Ads ad group.
package adgroup_update

import (
	"fmt"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	gads "flomation.app/automate/executor/actions/marketing/google_ads"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Ad Group: Update"
	Description  = "Change a Google Ads ad group's name, status or bids."
	Summary      = "Update a Google Ads ad group"
	Website      = "https://www.flomation.co"
	Icon         = "googleads+pencil"
	Date         = "12/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google Ads Connection", Placeholder: "${credentials.MyGoogleAds}", Required: true},
	{Name: "customer_id", Type: core.ConnectionTypeString, Label: "Customer ID (the ad account, e.g. 123-456-7890)", Required: true},
	{Name: "login_customer_id", Type: core.ConnectionTypeString, Label: "Manager Account ID (only when acting through an MCC)", Placeholder: "${credentials.MyGoogleAds.login_customer_id}"},
	{Name: "ad_group_id", Type: core.ConnectionTypeString, Label: "Ad Group ID", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "New Ad Group Name"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status", Options: []core.ConnectionOption{{Name: "Leave unchanged", Value: ""}, {Name: "Enabled", Value: "ENABLED"}, {Name: "Paused", Value: "PAUSED"}}},
	{Name: "max_cpc", Type: core.ConnectionTypeMoney, Label: "Maximum CPC Bid"},
	{Name: "target_cpa", Type: core.ConnectionTypeMoney, Label: "Target CPA"},
	{Name: "validate_only", Type: core.ConnectionTypeBoolean, Label: "Dry run — validate without changing anything"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Ad Group ID"},
	{Name: "resource_name", Type: core.ConnectionTypeString, Label: "Ad Group Resource Name"},
	{Name: "resource_names", Type: core.ConnectionTypeObject, Label: "Changed Resource Names"},
	{Name: "updated_fields", Type: core.ConnectionTypeObject, Label: "Fields Changed"},
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

	adGroup := map[string]interface{}{
		"resourceName": fmt.Sprintf("customers/%s/adGroups/%s", customerID, adGroupID),
	}
	var mask []string

	if v := gads.OptionalString("name", inputs); v != "" {
		adGroup["name"] = v
		mask = append(mask, "name")
	}
	if v := strings.ToUpper(gads.OptionalString("status", inputs)); v != "" {
		adGroup["status"] = v
		mask = append(mask, "status")
	}

	bid, err := gads.MoneyToMicros("max_cpc", inputs)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}
	if bid != nil {
		adGroup["cpcBidMicros"] = strconv.FormatInt(*bid, 10)
		mask = append(mask, "cpc_bid_micros")
	}

	cpa, err := gads.MoneyToMicros("target_cpa", inputs)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}
	if cpa != nil {
		adGroup["targetCpaMicros"] = strconv.FormatInt(*cpa, 10)
		mask = append(mask, "target_cpa_micros")
	}

	if len(mask) == 0 {
		return gads.ErrorResult("nothing to update — set at least one field to change"), nil
	}

	operations := []interface{}{map[string]interface{}{
		"update":     adGroup,
		"updateMask": strings.Join(mask, ","),
	}}
	opt := gads.MutateOptions{ValidateOnly: gads.OptionalBool("validate_only", inputs), PartialFailure: false}

	resp, err := gads.NewClient(token, login).Mutate(flow, customerID, "adGroups", operations, opt)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}
	return gads.MutateResult(resp, opt,
		fmt.Sprintf("Updated ad group %s (%s)", adGroupID, strings.Join(mask, ", ")),
		map[string]interface{}{"updated_fields": mask}), nil
}
