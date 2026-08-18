// Package ad_get reads a single Meta ad back by ID.
package ad_get

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	meta "flomation.app/automate/executor/actions/marketing/meta_ads"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Ads: Get"
	Description  = "Read a Meta ad by ID, including its ad set, creative and effective status."
	Website      = "https://www.flomation.co"
	Icon         = "facebook+eye"
	Date         = "18/08/2026"
	Type         = core.ActionTypeAction
)

// A by-ID read is not a nicety. Without one, the only way to answer "does this
// object exist and what is it attached to" is to list the account and filter —
// which cannot distinguish "not created" from "not returned by this listing",
// and invites blaming a propagation delay for either.
const defaultFields = "id,name,adset_id,campaign_id,creative,status,effective_status,created_time,updated_time"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Meta Access Token", Placeholder: "${secrets.MetaAdsToken}", Required: true},
	{Name: "app_secret", Type: core.ConnectionTypeSecret, Label: "App Secret (recommended)", Placeholder: "${secrets.MetaAppSecret}"},
	{Name: "ad_id", Type: core.ConnectionTypeString, Label: "Ad ID", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: defaultFields},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Ad"},
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

	resp, err := meta.NewClient(token, secret).Get(flow, "/"+id,
		url.Values{"fields": {meta.Fields("fields", inputs, defaultFields)}})
	if err != nil {
		return meta.ErrorResult(err.Error()), nil
	}

	return meta.ListResult([]map[string]interface{}{resp},
		fmt.Sprintf("Ad %s", id),
		map[string]interface{}{"result": resp, "id": id}), nil
}
