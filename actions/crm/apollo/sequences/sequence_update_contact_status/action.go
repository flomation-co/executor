package sequence_update_contact_status

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	apollo_common "flomation.app/automate/executor/actions/crm/apollo"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Sequence: Update Contact Status"
	Description  = "Mark finished / remove / stop contacts in an Apollo sequence. Master key required."
	Website      = "https://www.flomation.co"
	Icon         = "apollo+circle-check"
	Date         = "01/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Apollo API Key", Placeholder: "${secrets.ApolloApiKey} (master key)", Required: true},
	{Name: "sequence_id", Type: core.ConnectionTypeString, Label: "Sequence ID", Placeholder: "The Apollo sequence (emailer campaign) ID", Required: true},
	{Name: "contact_ids", Type: core.ConnectionTypeString, Label: "Contact IDs", Placeholder: "Comma-separated Apollo contact IDs", Required: true},
	{Name: "mode", Type: core.ConnectionTypeString, Label: "Mode", Placeholder: "mark_as_finished", Required: true, Options: []core.ConnectionOption{
		{Name: "Mark as finished", Value: "mark_as_finished"},
		{Name: "Remove from sequence", Value: "remove"},
		{Name: "Stop", Value: "stop"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Job ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Progress Job"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := apollo_common.GetAPIKey(inputs)
	if err != nil {
		return apollo_common.ErrorResult(err.Error()), nil
	}
	seqID, err := apollo_common.RequiredString("sequence_id", inputs)
	if err != nil {
		return apollo_common.ErrorResult("a sequence ID is required"), nil
	}
	contactIDs := apollo_common.StringList("contact_ids", inputs)
	if len(contactIDs) == 0 {
		return apollo_common.ErrorResult("at least one contact ID is required"), nil
	}
	mode, err := apollo_common.RequiredString("mode", inputs)
	if err != nil {
		return apollo_common.ErrorResult("a mode (mark_as_finished/remove/stop) is required"), nil
	}

	// This endpoint takes its args as QUERY params (array forms), and the operation
	// is queued: it returns an entity_progress_job, not the final state.
	q := url.Values{}
	q.Add("emailer_campaign_ids[]", seqID)
	for _, c := range contactIDs {
		q.Add("contact_ids[]", c)
	}
	q.Set("mode", mode)

	resp, err := apollo_common.NewClient(apiKey).Request(flow, "POST", "/emailer_campaigns/remove_or_stop_contact_ids", q, nil)
	if err != nil {
		return apollo_common.MapError(err), nil
	}

	job := apollo_common.Obj(resp, "entity_progress_job")
	if job == nil {
		// Some responses may echo the job at the top level; fall back to the whole body.
		job = resp
	}
	return apollo_common.ObjectResult("", job, fmt.Sprintf("Queued '%s' for %d contact(s) in sequence %s", mode, len(contactIDs), seqID)), nil
}
