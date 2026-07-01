package mailchimp_campaign_send

import (
	"net/http"

	core "flomation.app/automate/executor"
	mailchimp "flomation.app/automate/executor/actions/mailchimp"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Campaign: Send"
	Description  = "Send a Mailchimp campaign immediately to its configured recipients."
	Website      = "https://www.flomation.co"
	Icon         = "mailchimp+paper-plane"
	Date         = "01/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Mailchimp API Key", Placeholder: "xxxxxxxx-us6", Required: true},
	{Name: "campaign_id", Type: core.ConnectionTypeString, Label: "Campaign ID", Required: true},
}

var Outputs = [...]core.Connection{
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

	if err := mailchimp.RequestNoContent(apiKey, http.MethodPost, mailchimp.CampaignPath(id)+"/actions/send", nil); err != nil {
		return mailchimp.ErrorResult(err.Error()), nil
	}
	return mailchimp.SuccessResult("Sent campaign " + id), nil
}
