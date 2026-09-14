// Package adgroup_create creates an ad group inside a Google Ads campaign.
package adgroup_create

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
	Name         = "Ad Group: Create"
	Description  = "Create a Google Ads ad group inside a campaign, with an optional maximum CPC bid."
	Summary      = "Create a Google Ads ad group"
	Website      = "https://www.flomation.co"
	Icon         = "googleads+plus"
	Date         = "12/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google Ads Connection", Placeholder: "${credentials.MyGoogleAds}", Required: true},
	{Name: "customer_id", Type: core.ConnectionTypeString, Label: "Customer ID (the ad account, e.g. 123-456-7890)", Required: true},
	{Name: "login_customer_id", Type: core.ConnectionTypeString, Label: "Manager Account ID (only when acting through an MCC)", Placeholder: "${credentials.MyGoogleAds.login_customer_id}"},
	{Name: "campaign_id", Type: core.ConnectionTypeString, Label: "Campaign ID", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Ad Group Name (must be unique in the campaign)", Required: true},
	{Name: "type", Type: core.ConnectionTypeString, Label: "Ad Group Type", Options: []core.ConnectionOption{{Name: "Search (standard)", Value: "SEARCH_STANDARD"}, {Name: "Display (standard)", Value: "DISPLAY_STANDARD"}}},
	{Name: "max_cpc", Type: core.ConnectionTypeMoney, Label: "Maximum CPC Bid (Manual CPC campaigns only)"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status on creation", Options: []core.ConnectionOption{{Name: "Enabled", Value: "ENABLED"}, {Name: "Paused", Value: "PAUSED"}}},
	{Name: "validate_only", Type: core.ConnectionTypeBoolean, Label: "Dry run — validate without creating anything"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Ad Group ID"},
	{Name: "resource_name", Type: core.ConnectionTypeString, Label: "Ad Group Resource Name"},
	{Name: "resource_names", Type: core.ConnectionTypeObject, Label: "Created Resource Names"},
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
	campaignID, err := gads.RequiredString("campaign_id", inputs)
	if err != nil {
		return gads.ErrorResult("a campaign ID is required"), nil
	}
	name, err := gads.RequiredString("name", inputs)
	if err != nil {
		return gads.ErrorResult("an ad group name is required"), nil
	}

	adGroup := map[string]interface{}{
		"name":     name,
		"campaign": fmt.Sprintf("customers/%s/campaigns/%s", customerID, campaignID),
	}

	status := strings.ToUpper(gads.OptionalString("status", inputs))
	if status == "" {
		// Unlike a campaign, an ad group enabled on creation spends nothing on
		// its own — its campaign's status governs that, and a new campaign from
		// this integration is paused. So ENABLED is the useful default here.
		status = "ENABLED"
	}
	adGroup["status"] = status

	if t := strings.ToUpper(gads.OptionalString("type", inputs)); t != "" {
		adGroup["type"] = t
	}

	bid, err := gads.MoneyToMicros("max_cpc", inputs)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}
	if bid != nil {
		// Only meaningful under Manual CPC; an automated bidding strategy on the
		// campaign ignores it rather than rejecting it, which is why this is not
		// validated against the campaign here.
		adGroup["cpcBidMicros"] = strconv.FormatInt(*bid, 10)
	}

	operations := []interface{}{map[string]interface{}{"create": adGroup}}
	opt := gads.MutateOptions{ValidateOnly: gads.OptionalBool("validate_only", inputs), PartialFailure: false}

	resp, err := gads.NewClient(token, login).Mutate(flow, customerID, "adGroups", operations, opt)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}
	return gads.MutateResult(resp, opt, fmt.Sprintf("Created ad group %q in campaign %s", name, campaignID), nil), nil
}
