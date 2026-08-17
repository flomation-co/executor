// Package campaign_list lists campaigns in a Meta ad account.
package campaign_list

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	meta "flomation.app/automate/executor/actions/marketing/meta_ads"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Campaigns: List"
	Description  = "List campaigns in a Meta ad account, with status and budget."
	Website      = "https://www.flomation.co"
	Icon         = "facebook+list"
	Date         = "17/08/2026"
	Type         = core.ActionTypeAction
)

// effective_status is included alongside status because they routinely differ:
// Campaigns inherit their parent's state, so an object can be ACTIVE in its
// own right while paused in practice. Reporting only `status` makes a paused
// account look live.
const defaultFields = "id,name,objective,status,effective_status,daily_budget,lifetime_budget,budget_remaining,start_time,stop_time,created_time,updated_time"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Meta Access Token", Placeholder: "${secrets.MetaAdsToken}", Required: true},
	{Name: "app_secret", Type: core.ConnectionTypeSecret, Label: "App Secret (recommended)", Placeholder: "${secrets.MetaAppSecret}"},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Ad Account ID", Placeholder: "act_1234567890 or 1234567890", Required: true},
	{Name: "effective_status", Type: core.ConnectionTypeString, Label: "Filter by Status", Options: []core.ConnectionOption{{Name: "Any", Value: ""}, {Name: "Active", Value: "ACTIVE"}, {Name: "Paused", Value: "PAUSED"}, {Name: "Archived", Value: "ARCHIVED"}}},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: defaultFields},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "25"},
	{Name: "after", Type: core.ConnectionTypeString, Label: "Page Cursor (from a previous run)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Campaigns"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "next_cursor", Type: core.ConnectionTypeString, Label: "Next Page Cursor"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, secret, err := meta.GetAuth(inputs)
	if err != nil {
		return meta.ErrorResult(err.Error()), nil
	}
	account, err := meta.RequiredString("account_id", inputs)
	if err != nil {
		return meta.ErrorResult("an ad account ID is required"), nil
	}

	p := url.Values{"fields": {meta.Fields("fields", inputs, defaultFields)}}
	meta.SetParam(p, "limit", "limit", inputs)
	meta.SetParam(p, "after", "after", inputs)
	// Graph expects the status filter as a JSON array, not a bare value.
	if s := meta.OptionalString("effective_status", inputs); s != "" {
		p.Set("effective_status", `["`+s+`"]`)
	}

	resp, err := meta.NewClient(token, secret).Get(flow, meta.AccountPath(account)+"/campaigns", p)
	if err != nil {
		return meta.ErrorResult(err.Error()), nil
	}

	items := meta.Data(resp)
	next := meta.NextCursor(resp)
	summary := fmt.Sprintf("Found %d campaign(s)", len(items))
	if next != "" {
		summary += " (more pages available — pass next_cursor back in as the Page Cursor)"
	}
	return meta.ListResult(items, summary, map[string]interface{}{"next_cursor": next}), nil
}
