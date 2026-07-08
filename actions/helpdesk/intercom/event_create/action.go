package helpdesk_intercom_event_create

import (
	"net/http"
	"time"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Track Event"
	Description  = "Record a custom event on a contact's timeline in Intercom (e.g. ordered-item or plan-upgraded). Identify the person by Contact ID, External ID, or Email."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+bolt"
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
	{Name: "event_name", Type: core.ConnectionTypeString, Label: "Event Name", Placeholder: "ordered-item — short, past-tense, hyphenated names work best", Required: true},
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact ID", Placeholder: "The Intercom contact ID — provide this, an External ID, or an Email"},
	{Name: "external_id", Type: core.ConnectionTypeString, Label: "External ID", Placeholder: "Your own ID for this person, e.g. their ID in your database"},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email", Placeholder: "jane@acme.com"},
	{Name: "created_at", Type: core.ConnectionTypeDateTime, Label: "Occurred At", Placeholder: "When the event happened — leave empty for right now"},
	{Name: "metadata", Type: core.ConnectionTypeObject, Label: "Metadata (JSON)", Placeholder: `{"order_id":"12345","price":"$29.95"} — simple key/value details stored with the event`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Event ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Raw Response"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := intercom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	eventName, err := intercom.RequiredString("event_name", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	contactID := intercom.OptionalString("contact_id", inputs)
	externalID := intercom.OptionalString("external_id", inputs)
	email := intercom.OptionalString("email", inputs)
	if contactID == "" && externalID == "" && email == "" {
		return intercom.ErrorResult("provide a Contact ID, an External ID, or an Email so Intercom knows whose event this is"), nil
	}

	body := map[string]interface{}{"event_name": eventName}
	if contactID != "" {
		body["id"] = contactID
	}
	if externalID != "" {
		body["user_id"] = externalID
	}
	if email != "" {
		body["email"] = email
	}
	// Intercom requires created_at on every event — default to now, then let an
	// explicit Occurred At input overwrite it.
	body["created_at"] = time.Now().Unix()
	if err := intercom.SetUnixIfPresent(body, inputs, "created_at", "created_at"); err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	meta, err := intercom.OptionalJSON("metadata", inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	if meta != nil {
		obj, ok := meta.(map[string]interface{})
		if !ok {
			return intercom.ErrorResult(`metadata must be a JSON object, e.g. {"order_id":"12345"}`), nil
		}
		body["metadata"] = obj
	}

	// POST /events replies 202 with an EMPTY body (fire-and-forget), which
	// WriteObject decodes to an empty object — so there is no event ID to return.
	obj, err := intercom.WriteObject(auth, http.MethodPost, "/events", body, nil)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	return intercom.SuccessResult("", obj, "Event submitted"), nil
}
