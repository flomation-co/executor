package email_send

import (
	"fmt"

	core "flomation.app/automate/executor"
	apollo_common "flomation.app/automate/executor/actions/crm/apollo"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Email: Send Now"
	Description  = "Send a drafted Apollo email immediately (async). Master key required."
	Website      = "https://www.flomation.co"
	Icon         = "apollo+paper-plane"
	Date         = "01/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Apollo API Key", Placeholder: "${secrets.ApolloApiKey} (master key)", Required: true},
	{Name: "emailer_message_id", Type: core.ConnectionTypeString, Label: "Email Message ID", Placeholder: "The drafted email message ID to send", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Email Message ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Email Message"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := apollo_common.GetAPIKey(inputs)
	if err != nil {
		return apollo_common.ErrorResult(err.Error()), nil
	}
	id, err := apollo_common.RequiredString("emailer_message_id", inputs)
	if err != nil {
		return apollo_common.ErrorResult("an email message ID is required"), nil
	}

	// Send is queued (async): a 200 means accepted, not delivered.
	resp, err := apollo_common.NewClient(apiKey).Post(flow, "/emailer_messages/"+id+"/send_now", map[string]interface{}{})
	if err != nil {
		return apollo_common.MapError(err), nil
	}

	msg := apollo_common.Obj(resp, "emailer_message")
	if msg == nil {
		msg = resp
	}
	return apollo_common.ObjectResult(id, msg, fmt.Sprintf("Queued send for email %s", id)), nil
}
