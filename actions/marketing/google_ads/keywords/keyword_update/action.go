// Package keyword_update changes a keyword's bid or status.
package keyword_update

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
	Name         = "Keyword: Update"
	Description  = "Change a Google Ads keyword's bid or status. Match type cannot be changed."
	Summary      = "Change a keyword's bid or status"
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
	{Name: "criterion_id", Type: core.ConnectionTypeString, Label: "Keyword (Criterion) ID", Required: true},
	{Name: "max_cpc", Type: core.ConnectionTypeMoney, Label: "New Maximum CPC Bid"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status", Options: []core.ConnectionOption{{Name: "Leave unchanged", Value: ""}, {Name: "Enabled", Value: "ENABLED"}, {Name: "Paused", Value: "PAUSED"}, {Name: "Removed (permanent)", Value: "REMOVED"}}},
	{Name: "final_url", Type: core.ConnectionTypeString, Label: "Landing Page URL override"},
	{Name: "validate_only", Type: core.ConnectionTypeBoolean, Label: "Dry run — validate without changing anything"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Criterion ID"},
	{Name: "resource_name", Type: core.ConnectionTypeString, Label: "Keyword Resource Name"},
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
	criterionID, err := gads.RequiredString("criterion_id", inputs)
	if err != nil {
		return gads.ErrorResult("a keyword (criterion) ID is required — Keywords: List returns it as ad_group_criterion.criterion_id"), nil
	}

	// Like an ad, a keyword is addressed by the compound "<adGroupId>~<criterionId>".
	criterion := map[string]interface{}{
		"resourceName": fmt.Sprintf("customers/%s/adGroupCriteria/%s~%s", customerID, adGroupID, criterionID),
	}
	var mask []string

	bid, err := gads.MoneyToMicros("max_cpc", inputs)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}
	if bid != nil {
		criterion["cpcBidMicros"] = strconv.FormatInt(*bid, 10)
		mask = append(mask, "cpc_bid_micros")
	}
	if v := strings.ToUpper(gads.OptionalString("status", inputs)); v != "" {
		criterion["status"] = v
		mask = append(mask, "status")
	}
	if v := gads.OptionalString("final_url", inputs); v != "" {
		criterion["finalUrls"] = []string{v}
		mask = append(mask, "final_urls")
	}

	if len(mask) == 0 {
		// Match type is immutable in Google Ads: changing one means removing the
		// keyword and adding it again, which is a different (and destructive)
		// operation, so it is not offered here as if it were an edit.
		return gads.ErrorResult("nothing to update — set a bid, a status or a landing page. Match type cannot be changed on an existing keyword; remove it and add it again instead"), nil
	}

	operations := []interface{}{map[string]interface{}{
		"update":     criterion,
		"updateMask": strings.Join(mask, ","),
	}}
	opt := gads.MutateOptions{ValidateOnly: gads.OptionalBool("validate_only", inputs), PartialFailure: false}

	resp, err := gads.NewClient(token, login).Mutate(flow, customerID, "adGroupCriteria", operations, opt)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}
	return gads.MutateResult(resp, opt,
		fmt.Sprintf("Updated keyword %s (%s)", criterionID, strings.Join(mask, ", ")),
		map[string]interface{}{"updated_fields": mask}), nil
}
