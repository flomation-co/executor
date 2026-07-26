package crm_salesforce_quote_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Delete Quote"
	Description  = "Send a quote and its product lines to the Salesforce Recycle Bin, where an administrator can restore it for 15 days. Use it to clear out superseded drafts."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+trash"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "quote_id", Type: core.ConnectionTypeString, Label: "Quote ID", Placeholder: "0Q05f000000AbCdAAK - the quote to delete", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Quote ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Raw Response"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	id := salesforce.OptionalString("quote_id", inputs)
	if err := salesforce.ValidateRecordID(id); err != nil {
		return nil, err
	}

	// Deleting is not idempotent in Salesforce: a second run on the same ID comes
	// back 404 ENTITY_IS_DELETED (verified live). That is a provider answer, not a
	// wiring mistake, so it lands on the error port as data and a flow can branch
	// on it.
	if err := salesforce.DeleteRecord(instanceURL, token, "Quote", id); err != nil {
		if salesforce.ErrorHasCode(err, "INVALID_TYPE") {
			return salesforce.ErrorResult(fmt.Sprintf(
				"quotes are switched off in your Salesforce org — an administrator can turn them on under Setup ▸ Quotes ▸ Quote Settings (%s)", err.Error())), nil
		}
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Salesforce answers 204 No Content, so the ID we were given is the only
	// thing there is to hand downstream — which is what a "quote withdrawn, now
	// tidy up" flow needs. Deleting a quote takes its product lines with it, and
	// a quote that was syncing to its deal simply stops syncing (verified live:
	// a synced quote deletes without complaint).
	record := map[string]interface{}{"Id": id, "deleted": true}
	return salesforce.RecordResult(id, record, fmt.Sprintf("Deleted quote %s and its product lines — it is in the Salesforce Recycle Bin for 15 days", id)), nil
}
