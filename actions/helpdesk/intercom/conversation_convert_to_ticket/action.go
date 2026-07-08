package helpdesk_intercom_conversation_convert_to_ticket

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Convert Conversation to Ticket"
	Description  = "Turn an Intercom conversation into a ticket of the type you choose. The conversation itself becomes the ticket (they share the same ID) — deleting the ticket also deletes the conversation."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+arrow-right-arrow-left"
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
	{Name: "conversation_id", Type: core.ConnectionTypeString, Label: "Conversation ID", Placeholder: "The ID of the conversation to convert", Required: true},
	{Name: "ticket_type_id", Type: core.ConnectionTypeString, Label: "Ticket Type", Placeholder: "The type of ticket to create", Required: true},
	{Name: "attributes", Type: core.ConnectionTypeObject, Label: "Ticket Attributes (JSON)", Placeholder: `{"_default_title_":"Refund request"} — values for the ticket type's attributes`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Ticket ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Ticket"},
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
	ticketTypeID, err := intercom.RequiredString("ticket_type_id", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	body := map[string]interface{}{"ticket_type_id": ticketTypeID}
	if err := intercom.SetJSONIfPresent(body, inputs, "attributes", "attributes"); err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}

	obj, err := intercom.WriteObject(auth, http.MethodPost, "/conversations/"+url.PathEscape(id)+"/convert", body, nil)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	return intercom.ResourceResult(obj, "Converted conversation "+id+" to a ticket"), nil
}
