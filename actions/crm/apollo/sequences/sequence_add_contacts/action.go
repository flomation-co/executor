package sequence_add_contacts

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	apollo_common "flomation.app/automate/executor/actions/crm/apollo"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Sequence: Add Contacts"
	Description  = "Add contacts to an Apollo sequence from a sending mailbox."
	Website      = "https://www.flomation.co"
	Icon         = "apollo+user-plus"
	Date         = "01/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Apollo API Key", Placeholder: "${secrets.ApolloApiKey}", Required: true},
	{Name: "sequence_id", Type: core.ConnectionTypeString, Label: "Sequence ID", Placeholder: "The Apollo sequence (emailer campaign) ID", Required: true},
	{Name: "send_email_from_email_account_id", Type: core.ConnectionTypeString, Label: "Send From (Mailbox ID)", Placeholder: "The connected mailbox ID to send from", Required: true},
	{Name: "contact_ids", Type: core.ConnectionTypeString, Label: "Contact IDs", Placeholder: "Comma-separated Apollo contact IDs"},
	{Name: "label_names", Type: core.ConnectionTypeString, Label: "List Names", Placeholder: "Alternatively add all contacts on these lists (comma-separated)"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status", Placeholder: "active", Options: []core.ConnectionOption{
		{Name: "Active", Value: "active"},
		{Name: "Paused", Value: "paused"},
	}},
	{Name: "sequence_no_email", Type: core.ConnectionTypeBoolean, Label: "Skip Contacts Without Email"},
	{Name: "sequence_unverified_email", Type: core.ConnectionTypeBoolean, Label: "Allow Unverified Emails"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Added Contacts"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := apollo_common.GetAPIKey(inputs)
	if err != nil {
		return apollo_common.ErrorResult(err.Error()), nil
	}
	sequenceID, err := apollo_common.RequiredString("sequence_id", inputs)
	if err != nil {
		return apollo_common.ErrorResult("a sequence ID is required"), nil
	}
	mailboxID, err := apollo_common.RequiredString("send_email_from_email_account_id", inputs)
	if err != nil {
		return apollo_common.ErrorResult("a sending mailbox ID (send_email_from_email_account_id) is required"), nil
	}

	contactIDs := apollo_common.StringList("contact_ids", inputs)
	labelNames := apollo_common.StringList("label_names", inputs)
	if len(contactIDs) == 0 && len(labelNames) == 0 {
		return apollo_common.ErrorResult("provide contact_ids or label_names to add to the sequence"), nil
	}

	// emailer_campaign_id and the sending mailbox are query params per Apollo's
	// add_contact_ids contract; contact selection travels in the body.
	q := url.Values{}
	q.Set("emailer_campaign_id", sequenceID)
	q.Set("send_email_from_email_account_id", mailboxID)

	body := map[string]interface{}{}
	if len(contactIDs) > 0 {
		body["contact_ids"] = contactIDs
	}
	if len(labelNames) > 0 {
		body["label_names"] = labelNames
	}
	apollo_common.SetString(body, "status", "status", inputs)
	apollo_common.SetBool(body, "sequence_no_email", "sequence_no_email", inputs)
	apollo_common.SetBool(body, "sequence_unverified_email", "sequence_unverified_email", inputs)

	path := "/emailer_campaigns/" + sequenceID + "/add_contact_ids"
	resp, err := apollo_common.NewClient(apiKey).Request(flow, "POST", path, q, body)
	if err != nil {
		return apollo_common.MapError(err), nil
	}

	// Apollo echoes the added contacts under "contacts".
	contacts := apollo_common.Arr(resp, "contacts")
	return apollo_common.ListResult(contacts, fmt.Sprintf("Added %d contacts to the sequence", len(contacts))), nil
}
