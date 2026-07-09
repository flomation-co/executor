package marketing_sendgrid_list_remove_contacts

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
	Name         = "SendGrid: Remove Contacts from List"
	Description  = "Remove contacts from a SendGrid Marketing list without deleting them — they stay in your account and in any other lists. SendGrid processes the removal in the background and returns a job ID."
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
	{Name: "list_id", Type: core.ConnectionTypeString, Label: "List", Placeholder: "The contact list to remove contacts from — see \"SendGrid: List Contact Lists\"", Required: true},
	{Name: "contact_ids", Type: core.ConnectionTypeString, Label: "Contact IDs", Placeholder: "Comma-separated contact IDs to remove from the list", Required: true},
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

	listID, err := sendgrid.RequiredString("list_id", inputs)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	raw, err := sendgrid.RequiredString("contact_ids", inputs)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	ids := sendgrid.SplitCSV(raw)
	if ids == nil {
		return sendgrid.ErrorResult("contact_ids is required"), nil
	}
	q := url.Values{}
	q.Set("contact_ids", strings.Join(ids, ","))

	result, _, _, err := sendgrid.Do(auth, http.MethodDelete, "/v3/marketing/lists/"+url.PathEscape(listID)+"/contacts", q, nil)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	obj, _ := result.(map[string]interface{})
	jobID := sendgrid.StringifyID(obj["job_id"])
	return sendgrid.ResourceResult(jobID, obj, fmt.Sprintf("Removal of %d contact(s) from list %s accepted for processing (job %s)", len(ids), listID, jobID)), nil
}
