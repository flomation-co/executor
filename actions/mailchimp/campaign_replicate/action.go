package mailchimp_campaign_replicate

import (
	"net/http"

	core "flomation.app/automate/executor"
	mailchimp "flomation.app/automate/executor/actions/mailchimp"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Campaign: Replicate"
	Description  = "Create a copy of an existing Mailchimp campaign. Returns the newly created campaign."
	Website      = "https://www.flomation.co"
	Icon         = "mailchimp+copy"
	Date         = "01/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Mailchimp API Key", Placeholder: "xxxxxxxx-us6", Required: true},
	{Name: "campaign_id", Type: core.ConnectionTypeString, Label: "Campaign ID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Campaign ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Campaign"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := mailchimp.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}
	id, err := mailchimp.RequiredString("campaign_id", inputs)
	if err != nil {
		return mailchimp.ErrorResult(err.Error()), nil
	}

	m, err := mailchimp.Request(apiKey, http.MethodPost, mailchimp.CampaignPath(id)+"/actions/replicate", nil)
	if err != nil {
		return mailchimp.ErrorResult(err.Error()), nil
	}
	return mailchimp.ObjectResult(m, "Replicated campaign "+id), nil
}
