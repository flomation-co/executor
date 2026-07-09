package marketing_sendgrid_asm_group_create

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	sendgrid "flomation.app/automate/executor/actions/marketing/sendgrid"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "SendGrid: Create Unsubscribe Group"
	Description  = "Create an unsubscribe (ASM) group in SendGrid. Recipients who opt out of a group stop receiving email assigned to that group without unsubscribing from everything. Provide a name and a short description."
	Website      = "https://www.flomation.co"
	Icon         = "sendgrid+plus"
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
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "Weekly Digest — up to 30 characters, unique in your account", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "What recipients see when choosing what to unsubscribe from — up to 100 characters; SendGrid requires one when creating a group"},
	{Name: "is_default", Type: core.ConnectionTypeBoolean, Label: "Default Group", Placeholder: "Tick to make this the default group for emails sent without an unsubscribe group"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `Any other SendGrid field to include in the request body`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Group ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Unsubscribe Group"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := sendgrid.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	name, err := sendgrid.RequiredString("name", inputs)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{"name": name}
	sendgrid.SetIfPresent(body, inputs, "description", "description")
	sendgrid.SetBoolIfSet(body, inputs, "is_default", "is_default")
	if err := sendgrid.MergeAdditionalFields(body, inputs); err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}

	result, _, _, err := sendgrid.Do(auth, http.MethodPost, "/v3/asm/groups", nil, body)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	obj, _ := result.(map[string]interface{})
	return sendgrid.ResourceResult(sendgrid.StringifyID(obj["id"]), obj, fmt.Sprintf("Created unsubscribe group %q", name)), nil
}
