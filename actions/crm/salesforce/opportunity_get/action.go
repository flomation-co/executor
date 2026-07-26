package crm_salesforce_opportunity_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Opportunity"
	Description  = "Look up one deal by its Salesforce ID and return everything on it - value, stage, close date, owner and every custom field."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+magnifying-glass"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "opportunity_id", Type: core.ConnectionTypeString, Label: "Opportunity ID", Placeholder: "0065f00000AbCdEAAV - the deal's Salesforce ID", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Name,Amount,StageName - leave blank to return every field"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Opportunity ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Opportunity"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	id := salesforce.OptionalString("opportunity_id", inputs)
	// A malformed ID is a wiring mistake, not a Salesforce failure, so it is a
	// hard error — catching it here beats a MALFORMED_ID nobody can act on.
	if err := salesforce.ValidateRecordID(id); err != nil {
		return nil, err
	}

	// Blank fields means "everything readable": Salesforce omits the ?fields=
	// filter and returns the full sObject, custom fields included. That is the
	// useful default for a lookup — a deal's important field is usually a custom
	// one nobody thought to list.
	fields := salesforce.OptionalString("fields", inputs)
	// A misspelled field name — "Close Date" with the space, the label rather
	// than the API name, which is the commonest Salesforce mistake there is — is
	// rejected locally and never reaches Salesforce, so it is a configuration
	// mistake and takes the hard error return. Left to GetRecord it lands on the
	// soft error port as though Salesforce had refused a well-formed request,
	// and the flow carries on down its failure branch retrying a typo forever.
	// Same guard, same wording, as account_get and case_get.
	for _, f := range salesforce.SplitList(fields) {
		if _, err := salesforce.ValidateSOQLFieldName(f); err != nil {
			return nil, fmt.Errorf("Fields — %w", err)
		}
	}

	record, err := salesforce.GetRecord(instanceURL, token, "Opportunity", id, fields)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf("Retrieved opportunity %s", id)
	if name, ok := record["Name"].(string); ok && name != "" {
		summary = fmt.Sprintf("Retrieved opportunity %q (%s)", name, id)
	}
	return salesforce.RecordResult(id, record, summary), nil
}
