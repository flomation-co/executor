// Package keyword_add adds keywords to a Google Ads ad group.
package keyword_add

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
	Name         = "Keywords: Add"
	Description  = "Add keywords to a Google Ads ad group, one per line, with a match type."
	Summary      = "Add keywords to an ad group"
	Website      = "https://www.flomation.co"
	Icon         = "googleads+key"
	Date         = "12/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google Ads Connection", Placeholder: "${credentials.MyGoogleAds}", Required: true},
	{Name: "customer_id", Type: core.ConnectionTypeString, Label: "Customer ID (the ad account, e.g. 123-456-7890)", Required: true},
	{Name: "login_customer_id", Type: core.ConnectionTypeString, Label: "Manager Account ID (only when acting through an MCC)", Placeholder: "${credentials.MyGoogleAds.login_customer_id}"},
	{Name: "ad_group_id", Type: core.ConnectionTypeString, Label: "Ad Group ID", Required: true},
	{Name: "keywords", Type: core.ConnectionTypeText, Label: "Keywords, one per line", Required: true},
	{Name: "match_type", Type: core.ConnectionTypeString, Label: "Match Type", Required: true, Options: []core.ConnectionOption{{Name: "Exact — only this search, or a close variant", Value: "EXACT"}, {Name: "Phrase — searches that include this meaning", Value: "PHRASE"}, {Name: "Broad — anything related", Value: "BROAD"}}},
	{Name: "max_cpc", Type: core.ConnectionTypeMoney, Label: "Maximum CPC Bid for these keywords"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status on creation", Options: []core.ConnectionOption{{Name: "Enabled", Value: "ENABLED"}, {Name: "Paused", Value: "PAUSED"}}},
	{Name: "final_url", Type: core.ConnectionTypeString, Label: "Landing Page URL for these keywords (optional override)"},
	{Name: "validate_only", Type: core.ConnectionTypeBoolean, Label: "Dry run — validate without adding anything"},
	{Name: "partial_failure", Type: core.ConnectionTypeBoolean, Label: "Add the keywords that are valid even if some fail (on by default)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "resource_names", Type: core.ConnectionTypeObject, Label: "Created Keyword Resource Names"},
	{Name: "resource_name", Type: core.ConnectionTypeString, Label: "First Keyword Resource Name"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "First Keyword ID"},
	{Name: "requested", Type: core.ConnectionTypeInteger, Label: "Keywords Requested"},
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

	status := strings.ToUpper(gads.OptionalString("status", inputs))
	if status == "" {
		status = "ENABLED"
	}

	bid, err := gads.MoneyToMicros("max_cpc", inputs)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}
	finalURL := gads.OptionalString("final_url", inputs)

	adGroup := fmt.Sprintf("customers/%s/adGroups/%s", customerID, adGroupID)
	operations := make([]interface{}, 0, len(terms))
	for _, term := range terms {
		criterion := map[string]interface{}{
			"adGroup": adGroup,
			"status":  status,
			"keyword": map[string]interface{}{"text": term, "matchType": matchType},
		}
		if bid != nil {
			criterion["cpcBidMicros"] = strconv.FormatInt(*bid, 10)
		}
		if finalURL != "" {
			criterion["finalUrls"] = []string{finalURL}
		}
		operations = append(operations, map[string]interface{}{"create": criterion})
	}

	// Partial failure defaults ON for a bulk add. One keyword duplicating an
	// existing one is routine and should not throw away the other forty-nine;
	// the failures are still reported, rather than swallowed.
	opt := gads.MutateOptionsFrom(inputs, true)

	resp, err := gads.NewClient(token, login).Mutate(flow, customerID, "adGroupCriteria", operations, opt)
	if err != nil {
		// A partial failure returns both a response and an error: the valid
		// keywords WERE added, so report what happened rather than implying
		// nothing did.
		if resp != nil {
			names := gads.MutatedResourceNames(resp)
			added := 0
			for _, n := range names {
				if n != "" {
					added++
				}
			}
			return map[string]interface{}{
				"tool_result":    fmt.Sprintf("Added %d of %d keyword(s) to ad group %s. %s", added, len(terms), adGroupID, err.Error()),
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
		fmt.Sprintf("Added %d %s keyword(s) to ad group %s", len(terms), matchType, adGroupID),
		map[string]interface{}{"requested": len(terms)}), nil
}
