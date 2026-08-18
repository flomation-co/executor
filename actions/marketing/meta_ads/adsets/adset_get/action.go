// Package adset_get reads a single Meta ad set back by ID.
package adset_get

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	meta "flomation.app/automate/executor/actions/marketing/meta_ads"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Ad Sets: Get"
	Description  = "Read a Meta ad set by ID, including targeting, budget and bid settings."
	Website      = "https://www.flomation.co"
	Icon         = "facebook+eye"
	Date         = "18/08/2026"
	Type         = core.ActionTypeAction
)

// A by-ID read is not a nicety. Without one, the only way to answer "does this
// object exist and what is it attached to" is to list the account and filter —
// which cannot distinguish "not created" from "not returned by this listing",
// and invites blaming a propagation delay for either.
const defaultFields = "id,name,campaign_id,status,effective_status,daily_budget,lifetime_budget,billing_event,optimization_goal,bid_strategy,bid_amount,targeting,start_time,end_time"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Meta Access Token", Placeholder: "${secrets.MetaAdsToken}", Required: true},
	{Name: "app_secret", Type: core.ConnectionTypeSecret, Label: "App Secret (recommended)", Placeholder: "${secrets.MetaAppSecret}"},
	{Name: "adset_id", Type: core.ConnectionTypeString, Label: "Ad Set ID", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: defaultFields},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Ad Set"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Ad Set ID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, secret, err := meta.GetAuth(inputs)
	if err != nil {
		return meta.ErrorResult(err.Error()), nil
	}
	id, err := meta.RequiredString("adset_id", inputs)
	if err != nil {
		return meta.ErrorResult("a ad set ID is required"), nil
	}

	resp, err := meta.NewClient(token, secret).Get(flow, "/"+id,
		url.Values{"fields": {meta.Fields("fields", inputs, defaultFields)}})
	if err != nil {
		return meta.ErrorResult(err.Error()), nil
	}

	return meta.ListResult([]map[string]interface{}{resp},
		fmt.Sprintf("Ad Set %s", id),
		map[string]interface{}{"result": resp, "id": id}), nil
}
