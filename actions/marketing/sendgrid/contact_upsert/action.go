package marketing_sendgrid_contact_upsert

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	sendgrid "flomation.app/automate/executor/actions/marketing/sendgrid"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "SendGrid: Create or Update Contact"
	Description  = "Add a marketing contact to SendGrid, or update them if the email address already exists. SendGrid applies the change asynchronously and returns a job ID — the contact will not be visible immediately; poll \"SendGrid: Get Import Status\" with the job ID to see when it completes."
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
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email", Placeholder: "jane@example.com — identifies the contact; an existing contact with this address is updated", Required: true},
	{Name: "first_name", Type: core.ConnectionTypeString, Label: "First Name", Placeholder: "Jane"},
	{Name: "last_name", Type: core.ConnectionTypeString, Label: "Last Name", Placeholder: "Doe"},
	{Name: "alternate_emails", Type: core.ConnectionTypeString, Label: "Alternate Emails", Placeholder: "Comma-separated additional addresses for this contact (up to 5)"},
	{Name: "phone_number", Type: core.ConnectionTypeString, Label: "Phone Number", Placeholder: "+441234567890"},
	{Name: "address_line_1", Type: core.ConnectionTypeString, Label: "Address Line 1", Placeholder: "1 High Street"},
	{Name: "address_line_2", Type: core.ConnectionTypeString, Label: "Address Line 2", Placeholder: "Suite 4"},
	{Name: "city", Type: core.ConnectionTypeString, Label: "City", Placeholder: "London"},
	{Name: "state_province_region", Type: core.ConnectionTypeString, Label: "State / Province / Region", Placeholder: "Greater London"},
	{Name: "postal_code", Type: core.ConnectionTypeString, Label: "Postal Code", Placeholder: "SW1A 1AA"},
	{Name: "country", Type: core.ConnectionTypeString, Label: "Country", Placeholder: "United Kingdom"},
	{Name: "custom_fields", Type: core.ConnectionTypeObject, Label: "Custom Fields (JSON)", Placeholder: `{"e1_T":"VIP"} — keyed by custom field ID (not name); use "SendGrid: List Custom Fields" to find the IDs`},
	{Name: "list_ids", Type: core.ConnectionTypeString, Label: "List IDs", Placeholder: "Comma-separated marketing list IDs to add the contact to"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `Any other SendGrid contact field, e.g. {"external_id":"crm-123"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Job ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := sendgrid.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	email, err := sendgrid.RequiredString("email", inputs)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}

	contact := map[string]interface{}{"email": email}
	sendgrid.SetIfPresent(contact, inputs, "first_name", "first_name")
	sendgrid.SetIfPresent(contact, inputs, "last_name", "last_name")
	sendgrid.SetCSVIfPresent(contact, inputs, "alternate_emails", "alternate_emails")
	sendgrid.SetIfPresent(contact, inputs, "phone_number", "phone_number")
	sendgrid.SetIfPresent(contact, inputs, "address_line_1", "address_line_1")
	sendgrid.SetIfPresent(contact, inputs, "address_line_2", "address_line_2")
	sendgrid.SetIfPresent(contact, inputs, "city", "city")
	sendgrid.SetIfPresent(contact, inputs, "state_province_region", "state_province_region")
	sendgrid.SetIfPresent(contact, inputs, "postal_code", "postal_code")
	sendgrid.SetIfPresent(contact, inputs, "country", "country")
	customFields, err := sendgrid.OptionalJSON("custom_fields", inputs)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	if customFields != nil {
		obj, ok := customFields.(map[string]interface{})
		if !ok {
			return sendgrid.ErrorResult(`custom_fields must be a JSON object keyed by field ID, e.g. {"e1_T":"VIP"}`), nil
		}
		contact["custom_fields"] = obj
	}
	if err := sendgrid.MergeAdditionalFields(contact, inputs); err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{"contacts": []interface{}{contact}}
	if listIDs := sendgrid.SplitCSV(sendgrid.OptionalString("list_ids", inputs)); listIDs != nil {
		body["list_ids"] = listIDs
	}

	result, _, _, err := sendgrid.Do(auth, http.MethodPut, "/v3/marketing/contacts", nil, body)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	obj, ok := result.(map[string]interface{})
	if !ok {
		obj = map[string]interface{}{}
	}
	jobID := sendgrid.StringifyID(obj["job_id"])
	summary := fmt.Sprintf("Contact %s accepted for processing (job %s) — SendGrid applies it asynchronously; poll with \"SendGrid: Get Import Status\"", email, jobID)
	return sendgrid.ResourceResult(jobID, obj, summary), nil
}
