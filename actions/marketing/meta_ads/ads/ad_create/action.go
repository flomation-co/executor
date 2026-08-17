// Package ad_create creates a Meta ad, joining an ad set to a creative.
package ad_create

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	meta "flomation.app/automate/executor/actions/marketing/meta_ads"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Ads: Create"
	Description  = "Create a Meta ad from an existing ad set and creative."
	Website      = "https://www.flomation.co"
	Icon         = "facebook+plus"
	Date         = "17/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Meta Access Token", Placeholder: "${secrets.MetaAdsToken}", Required: true},
	{Name: "app_secret", Type: core.ConnectionTypeSecret, Label: "App Secret (recommended)", Placeholder: "${secrets.MetaAppSecret}"},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Ad Account ID", Placeholder: "act_1234567890", Required: true},
	{Name: "adset_id", Type: core.ConnectionTypeString, Label: "Ad Set ID", Required: true},
	{Name: "creative_id", Type: core.ConnectionTypeString, Label: "Creative ID (from Creatives: Create)", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Ad Name", Required: true},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status", Options: []core.ConnectionOption{
		{Name: "Paused (default — safe)", Value: "PAUSED"},
		{Name: "Active — enters review, then starts spending", Value: "ACTIVE"},
	}},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Additional Fields (JSON object)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Ad ID"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
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
	adsetID, err := meta.RequiredString("adset_id", inputs)
	if err != nil {
		return meta.ErrorResult("an ad set ID is required — an ad must belong to an ad set"), nil
	}
	creativeID, err := meta.RequiredString("creative_id", inputs)
	if err != nil {
		return meta.ErrorResult("a creative ID is required — create one with Creatives: Create first"), nil
	}
	name, err := meta.RequiredString("name", inputs)
	if err != nil {
		return meta.ErrorResult("an ad name is required"), nil
	}

	status := meta.OptionalString("status", inputs)
	if status == "" {
		status = "PAUSED"
	}

	p := url.Values{"name": {name}, "adset_id": {adsetID}, "status": {status}}
	// The creative is referenced as a nested object, not a bare id parameter.
	if err := meta.SetJSONValue(p, "creative", map[string]interface{}{"creative_id": creativeID}); err != nil {
		return meta.ErrorResult(err.Error()), nil
	}
	if err := meta.MergeJSONFields(p, "fields", inputs); err != nil {
		return meta.ErrorResult(err.Error()), nil
	}

	resp, err := meta.NewClient(token, secret).Post(flow, meta.AccountPath(account)+"/ads", p)
	if err != nil {
		return meta.ErrorResult(err.Error()), nil
	}

	id, _ := resp["id"].(string)
	summary := fmt.Sprintf("Created ad %q (%s) in ad set %s with creative %s, status %s", name, id, adsetID, creativeID, status)
	if status == "PAUSED" {
		summary += ". It is PAUSED and will not spend until set to ACTIVE."
	} else {
		// Worth stating: ACTIVE does not mean running. Meta reviews the ad
		// first, and a rejection shows up in effective_status, not status.
		summary += ". Meta will review it before delivery begins — check effective_status for the outcome."
	}
	return meta.OkResult(summary, map[string]interface{}{"id": id, "status": status}), nil
}
