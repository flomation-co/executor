// Package creative_get reads a single Meta ad creative back by ID.
package creative_get

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	meta "flomation.app/automate/executor/actions/marketing/meta_ads"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Creatives: Get"
	Description  = "Read a Meta ad creative by ID, including its full object_story_spec."
	Website      = "https://www.flomation.co"
	Icon         = "facebook+eye"
	Date         = "18/08/2026"
	Type         = core.ActionTypeAction
)

// object_story_spec is the point of this action: when Meta rejects an ad with
// "the ad creative is invalid", the answer is almost always visible in the spec
// — an empty image_hash, a missing link, a page that is not what you expected.
// Without a way to read it back you are reduced to guessing or curling by hand.
const defaultFields = "id,name,status,object_story_spec,image_hash,image_url,thumbnail_url,call_to_action_type,effective_object_story_id"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Meta Access Token", Placeholder: "${secrets.MetaAdsToken}", Required: true},
	{Name: "app_secret", Type: core.ConnectionTypeSecret, Label: "App Secret (recommended)", Placeholder: "${secrets.MetaAppSecret}"},
	{Name: "creative_id", Type: core.ConnectionTypeString, Label: "Creative ID", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: defaultFields},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Creative"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Creative ID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, secret, err := meta.GetAuth(inputs)
	if err != nil {
		return meta.ErrorResult(err.Error()), nil
	}
	id, err := meta.RequiredString("creative_id", inputs)
	if err != nil {
		return meta.ErrorResult("a creative ID is required"), nil
	}

	resp, err := meta.NewClient(token, secret).Get(flow, "/"+id,
		url.Values{"fields": {meta.Fields("fields", inputs, defaultFields)}})
	if err != nil {
		return meta.ErrorResult(err.Error()), nil
	}

	return meta.ListResult([]map[string]interface{}{resp},
		fmt.Sprintf("Creative %s", id),
		map[string]interface{}{"result": resp, "id": id}), nil
}
