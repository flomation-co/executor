package marketing_sendgrid_contact_import_status

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
	Name         = "SendGrid: Get Import Status"
	Description  = "Check the progress of an asynchronous contact job — the job ID returned by \"SendGrid: Create or Update Contact\" or \"SendGrid: Delete Contacts\". The status is pending, completed, errored, or failed, with counts of what was processed and a link to any error details."
	Website      = "https://www.flomation.co"
	Icon         = "sendgrid+clock"
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
	{Name: "job_id", Type: core.ConnectionTypeString, Label: "Job ID", Placeholder: "The job ID returned by a contact create/update or delete", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Job ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Job"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := sendgrid.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	jobID, err := sendgrid.RequiredString("job_id", inputs)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}

	result, _, _, err := sendgrid.Do(auth, http.MethodGet, "/v3/marketing/contacts/imports/"+url.PathEscape(jobID), nil, nil)
	if err != nil {
		return sendgrid.ErrorResult(err.Error()), nil
	}
	obj, ok := result.(map[string]interface{})
	if !ok {
		return sendgrid.ErrorResult("unexpected SendGrid import status response shape"), nil
	}
	summary := fmt.Sprintf("Retrieved import job %s", jobID)
	if status, _ := obj["status"].(string); status != "" {
		summary = fmt.Sprintf("Import job %s is %s", jobID, status)
	}
	return sendgrid.ResourceResult(jobID, obj, summary), nil
}
