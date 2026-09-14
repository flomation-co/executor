// Package ad_create_responsive_search creates a responsive search ad.
package ad_create_responsive_search

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	gads "flomation.app/automate/executor/actions/marketing/google_ads"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Ad: Create Responsive Search Ad"
	Description  = "Create a responsive search ad from headlines and descriptions, one per line."
	Summary      = "Create a responsive search ad"
	Website      = "https://www.flomation.co"
	Icon         = "googleads+plus"
	Date         = "12/09/2026"
	Type         = core.ActionTypeAction
)

// Google's limits for a responsive search ad. Checked here rather than left to
// the API because the server-side rejection names an operation index and a
// field path, while "headline 4 is 34 characters, 4 over the limit" tells the
// caller — very often a model that has just written the copy — exactly what to
// shorten. Getting this feedback in tool_result rather than as an opaque
// failure is the difference between an agent fixing its own copy and giving up.
const (
	minHeadlines    = 3
	maxHeadlines    = 15
	maxHeadlineLen  = 30
	minDescriptions = 2
	maxDescriptions = 4
	maxDescLen      = 90
	maxPathLen      = 15
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google Ads Connection", Placeholder: "${credentials.MyGoogleAds}", Required: true},
	{Name: "customer_id", Type: core.ConnectionTypeString, Label: "Customer ID (the ad account, e.g. 123-456-7890)", Required: true},
	{Name: "login_customer_id", Type: core.ConnectionTypeString, Label: "Manager Account ID (only when acting through an MCC)", Placeholder: "${credentials.MyGoogleAds.login_customer_id}"},
	{Name: "ad_group_id", Type: core.ConnectionTypeString, Label: "Ad Group ID", Required: true},
	{Name: "final_url", Type: core.ConnectionTypeString, Label: "Landing Page URL", Required: true},
	{Name: "headlines", Type: core.ConnectionTypeText, Label: "Headlines, one per line (3 to 15, each up to 30 characters)", Required: true},
	{Name: "descriptions", Type: core.ConnectionTypeText, Label: "Descriptions, one per line (2 to 4, each up to 90 characters)", Required: true},
	{Name: "path1", Type: core.ConnectionTypeString, Label: "Display Path 1 (up to 15 characters, shown after the domain)"},
	{Name: "path2", Type: core.ConnectionTypeString, Label: "Display Path 2 (up to 15 characters)"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status on creation", Options: []core.ConnectionOption{{Name: "Enabled", Value: "ENABLED"}, {Name: "Paused", Value: "PAUSED"}}},
	{Name: "validate_only", Type: core.ConnectionTypeBoolean, Label: "Dry run — validate without creating anything"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Ad Group Ad ID"},
	{Name: "resource_name", Type: core.ConnectionTypeString, Label: "Ad Resource Name"},
	{Name: "resource_names", Type: core.ConnectionTypeObject, Label: "Created Resource Names"},
	{Name: "headline_count", Type: core.ConnectionTypeInteger, Label: "Headlines Used"},
	{Name: "description_count", Type: core.ConnectionTypeInteger, Label: "Descriptions Used"},
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
	finalURL, err := gads.RequiredString("final_url", inputs)
	if err != nil {
		return gads.ErrorResult("a landing page URL is required"), nil
	}
	if !strings.HasPrefix(finalURL, "http://") && !strings.HasPrefix(finalURL, "https://") {
		return gads.ErrorResult(fmt.Sprintf("the landing page URL must start with http:// or https:// — got %q", finalURL)), nil
	}

	headlines, err := textAssets(gads.OptionalString("headlines", inputs), "headline", minHeadlines, maxHeadlines, maxHeadlineLen)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}
	descriptions, err := textAssets(gads.OptionalString("descriptions", inputs), "description", minDescriptions, maxDescriptions, maxDescLen)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}

	rsa := map[string]interface{}{
		"headlines":    headlines,
		"descriptions": descriptions,
	}
	for _, path := range []string{"path1", "path2"} {
		value := gads.OptionalString(path, inputs)
		if value == "" {
			continue
		}
		if length := len([]rune(value)); length > maxPathLen {
			return gads.ErrorResult(fmt.Sprintf("display path %q is %d characters, over the %d character limit", value, length, maxPathLen)), nil
		}
		rsa[path] = value
	}

	status := strings.ToUpper(gads.OptionalString("status", inputs))
	if status == "" {
		status = "ENABLED"
	}

	adGroupAd := map[string]interface{}{
		"adGroup": fmt.Sprintf("customers/%s/adGroups/%s", customerID, adGroupID),
		"status":  status,
		"ad": map[string]interface{}{
			"finalUrls":          []string{finalURL},
			"responsiveSearchAd": rsa,
		},
	}

	operations := []interface{}{map[string]interface{}{"create": adGroupAd}}
	opt := gads.MutateOptions{ValidateOnly: gads.OptionalBool("validate_only", inputs), PartialFailure: false}

	resp, err := gads.NewClient(token, login).Mutate(flow, customerID, "adGroupAds", operations, opt)
	if err != nil {
		return gads.ErrorResult(err.Error()), nil
	}

	// A new ad enters review and serves nothing until Google approves it, which
	// is normal and takes up to a day — worth saying, because an ad that is
	// ENABLED but showing no impressions otherwise reads as a broken flow.
	summary := fmt.Sprintf("Created responsive search ad in ad group %s with %d headline(s) and %d description(s). It now enters Google's review, which can take up to a working day before it serves",
		adGroupID, len(headlines), len(descriptions))

	return gads.MutateResult(resp, opt, summary, map[string]interface{}{
		"headline_count":    len(headlines),
		"description_count": len(descriptions),
	}), nil
}

// textAssets splits a one-per-line input into the {text: ...} assets a
// responsive search ad wants, validating count and length as it goes.
//
// Blank lines are dropped rather than rejected: copy pasted from a document or
// generated by a model routinely carries them, and failing on whitespace would
// be pedantry rather than protection.
func textAssets(raw, label string, minimum, maximum, maxLen int) ([]map[string]interface{}, error) {
	var values []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			values = append(values, line)
		}
	}

	if len(values) < minimum {
		return nil, fmt.Errorf("a responsive search ad needs at least %d %ss, one per line — got %d", minimum, label, len(values))
	}
	if len(values) > maximum {
		return nil, fmt.Errorf("a responsive search ad takes at most %d %ss — got %d", maximum, label, len(values))
	}

	// Report EVERY over-length line, not just the first: a model that has
	// written fifteen headlines should be able to fix them all in one pass
	// rather than discovering them one failed call at a time.
	var tooLong []string
	for i, value := range values {
		// Runes, not bytes — Google counts characters, so an em dash or an
		// accented word is one character, and a byte count would reject copy
		// that is actually within the limit.
		if length := len([]rune(value)); length > maxLen {
			tooLong = append(tooLong, fmt.Sprintf("%s %d is %d characters (%d over): %q", label, i+1, length, length-maxLen, value))
		}
	}
	if len(tooLong) > 0 {
		return nil, fmt.Errorf("%d %s(s) are over the %d character limit:\n  - %s", len(tooLong), label, maxLen, strings.Join(tooLong, "\n  - "))
	}

	assets := make([]map[string]interface{}, 0, len(values))
	for _, value := range values {
		assets = append(assets, map[string]interface{}{"text": value})
	}
	return assets, nil
}
