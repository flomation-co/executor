package marketing_sendgrid_contact_list

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	sendgrid "flomation.app/automate/executor/actions/marketing/sendgrid"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "SendGrid: List Recent Contacts"
	Description  = "Retrieve a sample of up to 50 most recently updated marketing contacts. SendGrid does not offer a way to page through every contact here — use \"SendGrid: Search Contacts\" to find specific contacts, or \"SendGrid: Count Contacts\" for the account total."
	Website      = "https://www.flomation.co"
	Icon         = "sendgrid+list"
	Date         = "09/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Placeholder: "SendGrid API key (SendGrid → Settings → API Keys), e.g. ${secrets.sendgrid_api}", Required: true},
	{
		Name:  "region",
		Type:  core.ConnectionTypeString,
		Label: "Region",
		Options: []core.ConnectionOption{
			{Name: "Global", Value: ""},
			{Name: "EU (data residency)", Value: "eu"},
		},
		Placeholder: "Global unless your account uses an EU regional subuser — the EU host has no Marketing endpoints (contacts, lists, segments)",
	},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Contacts"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Account Total"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := sendgrid.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	result, _, _, err := sendgrid.Do(auth, http.MethodGet, "/v3/marketing/contacts", nil, nil)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	obj, ok := result.(map[string]interface{})
	if !ok {
		return sendgrid.ErrorResult("unexpected SendGrid contacts response shape"), nil
	}
	items, _ := obj["result"].([]interface{})
	count := len(items)
	if total, ok := obj["contact_count"].(float64); ok {
		count = int(total)
	}
	summary := fmt.Sprintf("Retrieved a sample of %d recently updated contact(s) out of %d in the account", len(items), count)
	return sendgrid.ListResult(items, count, summary), nil
}
