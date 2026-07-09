package marketing_sendgrid_list_get

import (
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	sendgrid "flomation.app/automate/executor/actions/marketing/sendgrid"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "SendGrid: Get Contact List"
	Description  = "Look up one contact list in SendGrid Marketing — its name and how many contacts it holds, optionally with a sample of up to 50 of its contacts."
	Website      = "https://www.flomation.co"
	Icon         = "sendgrid+eye"
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
	{Name: "list_id", Type: core.ConnectionTypeString, Label: "List", Placeholder: "The contact list to fetch — see \"SendGrid: List Contact Lists\"", Required: true},
	{Name: "contact_sample", Type: core.ConnectionTypeBoolean, Label: "Include Contact Sample", Placeholder: "Tick to also return a sample of up to 50 of the list's contacts"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "List ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "List"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := sendgrid.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	listID, err := sendgrid.RequiredString("list_id", inputs)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	q := url.Values{}
	if v, _ := sendgrid.OptionalBoolSet("contact_sample", inputs); v {
		q.Set("contact_sample", "true")
	}

	result, _, _, err := sendgrid.Do(auth, http.MethodGet, "/v3/marketing/lists/"+url.PathEscape(listID), q, nil)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	obj, ok := result.(map[string]interface{})
	if !ok {
		return sendgrid.ErrorResult("unexpected SendGrid response shape"), nil
	}
	label := sendgrid.StringifyID(obj["id"])
	if name, _ := obj["name"].(string); name != "" {
		label = name
	}
	return sendgrid.ResourceResult(sendgrid.StringifyID(obj["id"]), obj, fmt.Sprintf("Retrieved list %q", label)), nil
}
