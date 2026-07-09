package marketing_sendgrid_list_update

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
	Name         = "SendGrid: Update Contact List"
	Description  = "Rename a contact list in SendGrid Marketing. Choose the list and provide its new name."
	Website      = "https://www.flomation.co"
	Icon         = "sendgrid+pencil"
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
	{Name: "list_id", Type: core.ConnectionTypeString, Label: "List", Placeholder: "The contact list to rename — see \"SendGrid: List Contact Lists\"", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "New Name", Placeholder: "The list's new name — must be unique in your account", Required: true},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `Any other SendGrid list field as JSON, e.g. {"key":"value"}`},
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
	name, err := sendgrid.RequiredString("name", inputs)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	body := map[string]interface{}{"name": name}
	if err := sendgrid.MergeAdditionalFields(body, inputs); err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}

	result, _, _, err := sendgrid.Do(auth, http.MethodPatch, "/v3/marketing/lists/"+url.PathEscape(listID), nil, body)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	obj, ok := result.(map[string]interface{})
	if !ok {
		return sendgrid.ErrorResult("unexpected SendGrid response shape"), nil
	}
	return sendgrid.ResourceResult(sendgrid.StringifyID(obj["id"]), obj, fmt.Sprintf("Renamed list %s to %q", listID, name)), nil
}
