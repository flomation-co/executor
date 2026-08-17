// Package audience_create creates a Meta custom audience.
package audience_create

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	meta "flomation.app/automate/executor/actions/marketing/meta_ads"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Audiences: Create"
	Description  = "Create a Meta custom audience to populate with hashed customer data."
	Website      = "https://www.flomation.co"
	Icon         = "facebook+people-group"
	Date         = "17/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Meta Access Token", Placeholder: "${secrets.MetaAdsToken}", Required: true},
	{Name: "app_secret", Type: core.ConnectionTypeSecret, Label: "App Secret (recommended)", Placeholder: "${secrets.MetaAppSecret}"},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Ad Account ID", Placeholder: "act_1234567890", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Audience Name", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description"},
	// Uploading customer data to Meta requires the advertiser to assert they
	// have the right to do so. Meta rejects the create without it, and the
	// requirement is a legal one rather than a formality, so it is surfaced as
	// an explicit input rather than silently defaulted.
	{Name: "customer_file_source", Type: core.ConnectionTypeString, Label: "Where did this data come from?", Required: true, Options: []core.ConnectionOption{
		{Name: "Provided directly by the customers", Value: "USER_PROVIDED_ONLY"},
		{Name: "From partners or other sources", Value: "PARTNER_PROVIDED_ONLY"},
		{Name: "A mix of both", Value: "BOTH_USER_AND_PARTNER_PROVIDED"},
	}},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Additional Fields (JSON object)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Custom Audience ID"},
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
		return meta.ErrorResult("an audience name is required"), nil
	}
	source, err := meta.RequiredString("customer_file_source", inputs)
	if err != nil {
		return meta.ErrorResult("Meta requires you to declare where the customer data came from"), nil
	}

	p := url.Values{
		"name":                 {name},
		"subtype":              {"CUSTOM"},
		"customer_file_source": {source},
	}
	meta.SetParam(p, "description", "description", inputs)
	if err := meta.MergeJSONFields(p, "fields", inputs); err != nil {
		return meta.ErrorResult(err.Error()), nil
	}

	resp, err := meta.NewClient(token, secret).Post(flow, meta.AccountPath(account)+"/customaudiences", p)
	if err != nil {
		return meta.ErrorResult(err.Error()), nil
	}

	id, _ := resp["id"].(string)
	return meta.OkResult(
		fmt.Sprintf("Created custom audience %q (%s). It is empty — add people with Audiences: Add People.", name, id),
		map[string]interface{}{"id": id},
	), nil
}
