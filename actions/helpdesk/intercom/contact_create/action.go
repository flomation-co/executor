package helpdesk_intercom_contact_create

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Create Contact"
	Description  = "Create a person in Intercom as a user or a lead. Provide at least an Email or an External ID so Intercom can identify them; everything else is optional."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+plus"
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
	{
		Name:  "role",
		Type:  core.ConnectionTypeString,
		Label: "Role",
		Options: []core.ConnectionOption{
			{Name: "User", Value: "user"},
			{Name: "Lead", Value: "lead"},
		},
	},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email", Placeholder: "jane@acme.com — provide this or an External ID"},
	{Name: "external_id", Type: core.ConnectionTypeString, Label: "External ID", Placeholder: "Your own ID for this person, e.g. their ID in your database"},
	{Name: "phone", Type: core.ConnectionTypeString, Label: "Phone", Placeholder: "+15551234567 (include the country code)"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "Jane Doe"},
	{Name: "avatar_url", Type: core.ConnectionTypeString, Label: "Avatar URL", Placeholder: "https://… — an image shown on the contact's profile"},
	{Name: "signed_up_at", Type: core.ConnectionTypeDateTime, Label: "Signed Up At", Placeholder: "When they signed up, e.g. 2026-07-08T09:00:00Z"},
	{Name: "last_seen_at", Type: core.ConnectionTypeDateTime, Label: "Last Seen At", Placeholder: "When they were last active"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Owner", Placeholder: "The teammate who owns this contact"},
	{Name: "unsubscribed_from_emails", Type: core.ConnectionTypeBoolean, Label: "Unsubscribed From Emails", Placeholder: "Tick to opt this person out of email"},
	{Name: "custom_attributes", Type: core.ConnectionTypeObject, Label: "Custom Attributes (JSON)", Placeholder: `{"plan_tier":"gold"} — attributes must already exist in Intercom (Settings → Data)`},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `Any other Intercom contact field, e.g. {"unsubscribed_from_sms":true}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Contact ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Contact"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := intercom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	email := intercom.OptionalString("email", inputs)
	externalID := intercom.OptionalString("external_id", inputs)
	if email == "" && externalID == "" {
		return intercom.ErrorResult("provide an Email or an External ID so Intercom can identify the contact"), nil
	}

	role := intercom.OptionalString("role", inputs)
	if role == "" {
		role = "user"
	}
	body := map[string]interface{}{"role": role}
	if email != "" {
		body["email"] = email
	}
	if externalID != "" {
		body["external_id"] = externalID
	}
	intercom.SetIfPresent(body, inputs, "phone", "phone")
	intercom.SetIfPresent(body, inputs, "name", "name")
	intercom.SetIfPresent(body, inputs, "avatar", "avatar_url")
	if err := intercom.SetUnixIfPresent(body, inputs, "signed_up_at", "signed_up_at"); err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	if err := intercom.SetUnixIfPresent(body, inputs, "last_seen_at", "last_seen_at"); err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	// owner_id is typed as a JSON integer by Intercom — a string is rejected.
	intercom.SetNumericIDIfPresent(body, inputs, "owner_id", "owner_id")
	intercom.SetBoolIfSet(body, inputs, "unsubscribed_from_emails", "unsubscribed_from_emails")
	if err := intercom.SetJSONIfPresent(body, inputs, "custom_attributes", "custom_attributes"); err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	if err := intercom.MergeAdditionalFields(body, inputs); err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}

	obj, err := intercom.WriteObject(auth, http.MethodPost, "/contacts", body, nil)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	label := email
	if label == "" {
		label = externalID
	}
	return intercom.ResourceResult(obj, fmt.Sprintf("Created %s %s", role, label)), nil
}
