// Package negative_keyword_add excludes search terms at ad group or campaign
// level.
package negative_keyword_add

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	gads "flomation.app/automate/executor/actions/marketing/google_ads"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Negative Keywords: Add"
	Description  = "Stop ads showing for these search terms, at campaign or ad group level."
	Summary      = "Add negative keywords"
	Website      = "https://www.flomation.co"
	Icon         = "googleads+ban"
	Date         = "12/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google Ads Connection", Placeholder: "${credentials.MyGoogleAds}", Required: true},
	{Name: "customer_id", Type: core.ConnectionTypeString, Label: "Customer ID (the ad account, e.g. 123-456-7890)", Required: true},
	{Name: "login_customer_id", Type: core.ConnectionTypeString, Label: "Manager Account ID (only when acting through an MCC)", Placeholder: "${credentials.MyGoogleAds.login_customer_id}"},
	{Name: "level", Type: core.ConnectionTypeString, Label: "Apply to", Required: true, Options: []core.ConnectionOption{{Name: "Campaign — blocks the whole campaign", Value: "CAMPAIGN"}, {Name: "Ad group — blocks one ad group only", Value: "AD_GROUP"}}},
	{Name: "campaign_id", Type: core.ConnectionTypeString, Label: "Campaign ID (for campaign level)"},
	{Name: "ad_group_id", Type: core.ConnectionTypeString, Label: "Ad Group ID (for ad group level)"},
	{Name: "keywords", Type: core.ConnectionTypeText, Label: "Search terms to exclude, one per line", Required: true, Placeholder: "${search_terms}"},
	{Name: "match_type", Type: core.ConnectionTypeString, Label: "Match Type", Required: true, Options: []core.ConnectionOption{{Name: "Exact — block only this exact search", Value: "EXACT"}, {Name: "Phrase — block searches containing this phrase", Value: "PHRASE"}, {Name: "Broad — block searches containing all these words", Value: "BROAD"}}},
	{Name: "validate_only", Type: core.ConnectionTypeBoolean, Label: "Dry run — validate without adding anything"},
	{Name: "partial_failure", Type: core.ConnectionTypeBoolean, Label: "Add the terms that are valid even if some fail (on by default)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "resource_names", Type: core.ConnectionTypeObject, Label: "Created Negative Keyword Resource Names"},
	{Name: "resource_name", Type: core.ConnectionTypeString, Label: "First Resource Name"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "First Criterion ID"},
	{Name: "requested", Type: core.ConnectionTypeInteger, Label: "Terms Requested"},
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

	matchType := strings.ToUpper(gads.OptionalString("match_type", inputs))
	switch matchType {
	case "EXACT", "PHRASE", "BROAD":
	case "":
		return gads.ErrorResult("a match type is required (Exact, Phrase or Broad)"), nil
	default:
		return gads.ErrorResult(fmt.Sprintf("%q is not a match type; use EXACT, PHRASE or BROAD", matchType)), nil
	}

	terms, err := gads.KeywordTerms(gads.OptionalString("keywords", inputs))
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}

	// Campaign-level and ad-group-level negatives live in DIFFERENT resources
	// (campaignCriteria vs adGroupCriteria) with different parent fields, which
	// is why the level is an explicit choice rather than inferred from whichever
	// id happens to be filled in.
	level := strings.ToUpper(gads.OptionalString("level", inputs))
	var resource, parentField, parent, scope string
	switch level {
	case "CAMPAIGN":
		campaignID, err := gads.RequiredString("campaign_id", inputs)
		if err != nil {
			return gads.ErrorResult("a campaign ID is required to add campaign-level negative keywords"), nil
		}
		resource, parentField = "campaignCriteria", "campaign"
		parent = fmt.Sprintf("customers/%s/campaigns/%s", customerID, campaignID)
		scope = "campaign " + campaignID
	case "AD_GROUP":
		adGroupID, err := gads.RequiredString("ad_group_id", inputs)
		if err != nil {
			return gads.ErrorResult("an ad group ID is required to add ad-group-level negative keywords"), nil
		}
		resource, parentField = "adGroupCriteria", "adGroup"
		parent = fmt.Sprintf("customers/%s/adGroups/%s", customerID, adGroupID)
		scope = "ad group " + adGroupID
	case "":
		return gads.ErrorResult("choose whether to apply these at campaign or ad group level"), nil
	default:
		return gads.ErrorResult(fmt.Sprintf("%q is not a level; use CAMPAIGN or AD_GROUP", level)), nil
	}

	operations := make([]interface{}, 0, len(terms))
	for _, term := range terms {
		operations = append(operations, map[string]interface{}{"create": map[string]interface{}{
			parentField: parent,
			// negative is the entire difference between a keyword you bid on
			// and one you block. Omitting it silently creates a POSITIVE
			// keyword, so the account starts bidding on exactly the terms
			// someone was trying to stop paying for.
			"negative": true,
			"keyword":  map[string]interface{}{"text": term, "matchType": matchType},
		}})
	}

	opt := gads.MutateOptionsFrom(inputs, true)

	resp, err := gads.NewClient(token, login).Mutate(flow, customerID, resource, operations, opt)
	if err != nil {
		if resp != nil {
			names := gads.MutatedResourceNames(resp)
			added := 0
			for _, n := range names {
				if n != "" {
					added++
				}
			}
			return map[string]interface{}{
				"tool_result":    fmt.Sprintf("Excluded %d of %d term(s) on %s. %s", added, len(terms), scope, err.Error()),
				"resource_names": names,
				"requested":      len(terms),
				"count":          added,
				"dry_run":        opt.ValidateOnly,
				"success":        false,
				"error":          err.Error(),
			}, nil
		}
		return gads.ErrorResult(err.Error()), nil
	}

	return gads.MutateResult(resp, opt,
		fmt.Sprintf("Excluded %d search term(s) on %s as %s negatives", len(terms), scope, matchType),
		map[string]interface{}{"requested": len(terms)}), nil
}
