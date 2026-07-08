package helpdesk_intercom_conversation_update

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Update Conversation"
	Description  = "Change an Intercom conversation — mark it read, retitle it, link a company, or set custom attributes."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+pen"
	Date         = "08/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "Access Token", Placeholder: "Your Intercom access token (Developer Hub → Authentication)", Required: true},
	{
		Name:  "region",
		Type:  core.ConnectionTypeString,
		Label: "Region",
		Options: []core.ConnectionOption{
			{Name: "US (default)", Value: "us"},
			{Name: "Europe", Value: "eu"},
			{Name: "Australia", Value: "au"},
		},
	},
	{Name: "conversation_id", Type: core.ConnectionTypeString, Label: "Conversation ID", Placeholder: "The ID of the conversation to update", Required: true},
	{Name: "read", Type: core.ConnectionTypeBoolean, Label: "Mark Read", Placeholder: "Tick to mark the conversation as read"},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title", Placeholder: "A new title for the conversation"},
	{Name: "company_id", Type: core.ConnectionTypeString, Label: "Company ID", Placeholder: "The Intercom company ID to link (not your own company_id)"},
	{Name: "custom_attributes", Type: core.ConnectionTypeObject, Label: "Custom Attributes (JSON)", Placeholder: `{"issue_type":"billing"} — attributes must already exist in Intercom (Settings → Data)`},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: "Any other Intercom conversation field"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Conversation ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Conversation"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := intercom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	id, err := intercom.RequiredString("conversation_id", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	body := map[string]interface{}{}
	intercom.SetBoolIfSet(body, inputs, "read", "read")
	intercom.SetIfPresent(body, inputs, "title", "title")
	intercom.SetIfPresent(body, inputs, "company_id", "company_id")
	if err := intercom.SetJSONIfPresent(body, inputs, "custom_attributes", "custom_attributes"); err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	if err := intercom.MergeAdditionalFields(body, inputs); err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}

	obj, err := intercom.WriteObject(auth, http.MethodPut, "/conversations/"+url.PathEscape(id), body, nil)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	return intercom.ResourceResult(obj, "Updated conversation "+id), nil
}
