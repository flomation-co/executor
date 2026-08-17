// Package ad_update updates an existing Meta ad.
package ad_update

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	meta "flomation.app/automate/executor/actions/marketing/meta_ads"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Ads: Update"
	Description  = "Rename, pause or activate an existing Meta ad."
	Website      = "https://www.flomation.co"
	Icon         = "facebook+pen"
	Date         = "17/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Meta Access Token", Placeholder: "${secrets.MetaAdsToken}", Required: true},
	{Name: "app_secret", Type: core.ConnectionTypeSecret, Label: "App Secret (recommended)", Placeholder: "${secrets.MetaAppSecret}"},
	{Name: "ad_id", Type: core.ConnectionTypeString, Label: "Ad ID", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "New Name (optional)"},
	// Only `status` is writable. `effective_status` is Meta's computed view,
	// which folds in the parent's state and ad review outcome, so writing to it
	// is meaningless — setting an ad ACTIVE under a PAUSED campaign leaves it
	// effectively paused.
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status", Options: []core.ConnectionOption{
		{Name: "Leave unchanged", Value: ""},
		{Name: "Active — starts spending", Value: "ACTIVE"},
		{Name: "Paused", Value: "PAUSED"},
		{Name: "Archived", Value: "ARCHIVED"},
	}},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Ad Account ID (only needed when changing a budget)", Placeholder: "act_1234567890"},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Additional Fields (JSON object)", Placeholder: `{"bid_amount":200}`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Ad ID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, secret, err := meta.GetAuth(inputs)
	if err != nil {
		return meta.ErrorResult(err.Error()), nil
	}
	id, err := meta.RequiredString("ad_id", inputs)
	if err != nil {
		return meta.ErrorResult("a ad ID is required"), nil
	}

	client := meta.NewClient(token, secret)
	p := url.Values{}
	meta.SetParam(p, "name", "name", inputs)
	meta.SetParam(p, "status", "status", inputs)

	currency := ""

	if err := meta.MergeJSONFields(p, "fields", inputs); err != nil {
		return meta.ErrorResult(err.Error()), nil
	}

	// An empty update would still POST and report success, which reads as "the
	// change was applied" when nothing was asked for. Refuse instead.
	if len(p) == 0 {
		return meta.ErrorResult("nothing to update — set a name, status, budget or an Additional Fields override"), nil
	}

	if _, err := client.Post(flow, "/"+id, p); err != nil {
		return meta.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf("Updated ad %s", id)
	if s := meta.OptionalString("status", inputs); s != "" {
		summary += fmt.Sprintf(" (status set to %s)", s)
		if s == "ACTIVE" {
			summary += " — it can now spend"
		}
	}
	if currency != "" {
		summary += fmt.Sprintf(". Budget interpreted in %s.", currency)
	}
	return meta.OkResult(summary, map[string]interface{}{"id": id}), nil
}
