package marketing_sendgrid_list_delete

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
	Name         = "SendGrid: Delete Contact List"
	Description  = "Delete a contact list from SendGrid Marketing, optionally deleting its contacts too. The contacts survive unless you tick Also Delete Contacts; when they are deleted SendGrid processes the removal in the background and returns a job ID."
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
	{Name: "list_id", Type: core.ConnectionTypeString, Label: "List", Placeholder: "The contact list to delete — see \"SendGrid: List Contact Lists\"", Required: true},
	{Name: "delete_contacts", Type: core.ConnectionTypeBoolean, Label: "Also Delete Contacts", Placeholder: "Tick to also delete the list's contacts from your account, not just the list itself"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Job or List ID"},
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

	listID, err := sendgrid.RequiredString("list_id", inputs)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	q := url.Values{}
	if v, _ := sendgrid.OptionalBoolSet("delete_contacts", inputs); v {
		q.Set("delete_contacts", "true")
	}

	result, _, status, err := sendgrid.Do(auth, http.MethodDelete, "/v3/marketing/lists/"+url.PathEscape(listID), q, nil)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	// The API answers 200 {"job_id"} when the delete is processed
	// asynchronously (contacts being deleted too) and 204 empty when the list
	// is simply gone — both are success.
	obj, _ := result.(map[string]interface{})
	jobID := sendgrid.StringifyID(obj["job_id"])
	if status == http.StatusNoContent || jobID == "" {
		return sendgrid.SuccessResult(listID, fmt.Sprintf("Deleted list %s", listID)), nil
	}
	return sendgrid.ResourceResult(jobID, obj, fmt.Sprintf("Delete of list %s accepted for processing (job %s) — its contacts are being deleted in the background", listID, jobID)), nil
}
