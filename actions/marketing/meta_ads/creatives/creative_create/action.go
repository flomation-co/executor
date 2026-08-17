// Package creative_create creates a Meta ad creative — the image, copy and
// destination an ad actually shows.
package creative_create

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	meta "flomation.app/automate/executor/actions/marketing/meta_ads"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Creatives: Create"
	Description  = "Create a Meta ad creative from a Page, image, message and link."
	Website      = "https://www.flomation.co"
	Icon         = "facebook+image"
	Date         = "17/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Meta Access Token", Placeholder: "${secrets.MetaAdsToken}", Required: true},
	{Name: "app_secret", Type: core.ConnectionTypeSecret, Label: "App Secret (recommended)", Placeholder: "${secrets.MetaAppSecret}"},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Ad Account ID", Placeholder: "act_1234567890", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Creative Name", Required: true},
	// Every creative is published BY a Page — Meta has no concept of an ad
	// without one, so this is required rather than optional.
	{Name: "page_id", Type: core.ConnectionTypeString, Label: "Facebook Page ID", Required: true},
	{Name: "message", Type: core.ConnectionTypeText, Label: "Primary Text", Placeholder: "The copy shown above the image"},
	{Name: "headline", Type: core.ConnectionTypeString, Label: "Headline"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description"},
	{Name: "link", Type: core.ConnectionTypeString, Label: "Destination URL", Placeholder: "https://www.flomation.co"},
	{Name: "image_hash", Type: core.ConnectionTypeString, Label: "Image Hash (from Media: Upload Image)", Placeholder: "${node.image_hash}"},
	{Name: "call_to_action", Type: core.ConnectionTypeString, Label: "Call to Action", Options: []core.ConnectionOption{
		{Name: "None", Value: ""},
		{Name: "Learn More", Value: "LEARN_MORE"},
		{Name: "Sign Up", Value: "SIGN_UP"},
		{Name: "Shop Now", Value: "SHOP_NOW"},
		{Name: "Book Now", Value: "BOOK_TRAVEL"},
		{Name: "Contact Us", Value: "CONTACT_US"},
		{Name: "Download", Value: "DOWNLOAD"},
		{Name: "Get Quote", Value: "GET_QUOTE"},
		{Name: "Subscribe", Value: "SUBSCRIBE"},
	}},
	// The full object_story_spec is enormous (carousels, video, dynamic
	// formats). The curated inputs above build the common single-image link ad;
	// anything else is supplied whole here and wins outright.
	{Name: "object_story_spec", Type: core.ConnectionTypeText, Label: "Full Story Spec (JSON — overrides the fields above)", Placeholder: `{"page_id":"123","link_data":{...}}`},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Additional Fields (JSON object)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Creative ID"},
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
	name, err := meta.RequiredString("name", inputs)
	if err != nil {
		return meta.ErrorResult("a creative name is required"), nil
	}
	pageID, err := meta.RequiredString("page_id", inputs)
	if err != nil {
		return meta.ErrorResult("a Facebook Page ID is required — every ad creative is published by a Page"), nil
	}

	p := url.Values{"name": {name}}

	// An explicit story spec replaces the curated build entirely rather than
	// merging with it: half-applying two competing descriptions of the same
	// creative is how you get an ad that renders as neither.
	if spec := meta.OptionalString("object_story_spec", inputs); spec != "" {
		if err := meta.SetJSONParam(p, "object_story_spec", "object_story_spec", inputs); err != nil {
			return meta.ErrorResult(err.Error()), nil
		}
	} else {
		link := meta.OptionalString("link", inputs)
		if link == "" {
			return meta.ErrorResult("a Destination URL is required (or supply a full Story Spec instead)"), nil
		}
		linkData := map[string]interface{}{"link": link}
		if v := meta.OptionalString("message", inputs); v != "" {
			linkData["message"] = v
		}
		if v := meta.OptionalString("headline", inputs); v != "" {
			linkData["name"] = v
		}
		if v := meta.OptionalString("description", inputs); v != "" {
			linkData["description"] = v
		}
		if v := meta.OptionalString("image_hash", inputs); v != "" {
			linkData["image_hash"] = v
		}
		if cta := meta.OptionalString("call_to_action", inputs); cta != "" {
			linkData["call_to_action"] = map[string]interface{}{
				"type":  cta,
				"value": map[string]interface{}{"link": link},
			}
		}
		if err := meta.SetJSONValue(p, "object_story_spec", map[string]interface{}{
			"page_id":   pageID,
			"link_data": linkData,
		}); err != nil {
			return meta.ErrorResult(err.Error()), nil
		}
	}

	if err := meta.MergeJSONFields(p, "fields", inputs); err != nil {
		return meta.ErrorResult(err.Error()), nil
	}

	resp, err := meta.NewClient(token, secret).Post(flow, meta.AccountPath(account)+"/adcreatives", p)
	if err != nil {
		return meta.ErrorResult(err.Error()), nil
	}

	id, _ := resp["id"].(string)
	return meta.OkResult(
		fmt.Sprintf("Created creative %q (%s). Pass this ID to Ads: Create.", name, id),
		map[string]interface{}{"id": id},
	), nil
}
