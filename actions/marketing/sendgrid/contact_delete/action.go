package marketing_sendgrid_contact_delete

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	core "flomation.app/automate/executor"
	sendgrid "flomation.app/automate/executor/actions/marketing/sendgrid"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "SendGrid: Delete Contacts"
	Description  = "Delete marketing contacts from SendGrid, either by a list of contact IDs or every contact in the account. SendGrid deletes asynchronously and returns a job ID — poll \"SendGrid: Get Import Status\" with the job ID to see when it completes."
	Website      = "https://www.flomation.co"
	Icon         = "sendgrid+trash"
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
	{Name: "contact_ids", Type: core.ConnectionTypeString, Label: "Contact IDs", Placeholder: "Comma-separated SendGrid contact IDs to delete — leave blank when deleting all contacts"},
	{Name: "delete_all_contacts", Type: core.ConnectionTypeBoolean, Label: "Delete All Contacts", Placeholder: "Tick to delete EVERY contact in the account — leave Contact IDs blank when ticked"},
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

	ids := sendgrid.SplitCSV(sendgrid.OptionalString("contact_ids", inputs))
	deleteAll, _ := sendgrid.OptionalBoolSet("delete_all_contacts", inputs)
	// The API takes ids XOR delete_all_contacts — exactly one mode.
	if deleteAll && ids != nil {
		return sendgrid.ErrorResult("provide Contact IDs or tick Delete All Contacts, not both"), nil
	}
	if !deleteAll && ids == nil {
		return sendgrid.ErrorResult("provide Contact IDs to delete, or tick Delete All Contacts"), nil
	}

	query := url.Values{}
	if deleteAll {
		query.Set("delete_all_contacts", "true")
	} else {
		query.Set("ids", strings.Join(ids, ","))
	}

	result, _, _, err := sendgrid.Do(auth, http.MethodDelete, "/v3/marketing/contacts", query, nil)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	obj, ok := result.(map[string]interface{})
	if !ok {
		obj = map[string]interface{}{}
	}
	jobID := sendgrid.StringifyID(obj["job_id"])
	subject := fmt.Sprintf("Deletion of %d contact(s)", len(ids))
	if deleteAll {
		subject = "Deletion of ALL contacts"
	}
	summary := fmt.Sprintf("%s accepted for processing (job %s) — SendGrid deletes asynchronously; poll with \"SendGrid: Get Import Status\"", subject, jobID)
	return sendgrid.ResourceResult(jobID, obj, summary), nil
}
