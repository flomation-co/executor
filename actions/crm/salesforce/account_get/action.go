package crm_salesforce_account_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Account"
	Description  = "Look up one Salesforce account by its record ID and return its details. Leave Fields blank to get everything the connected Salesforce user is allowed to see."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+magnifying-glass"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Account", Placeholder: "Record ID of the account, e.g. 0015f00000AbCdEAAV", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Name, Phone, BillingCity (comma-separated; blank returns every field)"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Account ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Account"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	accountID := salesforce.OptionalString("account_id", inputs)
	if accountID == "" {
		return nil, fmt.Errorf("account_id is required — the record ID of the account to fetch, e.g. 0015f00000AbCdEAAV")
	}
	// Catch a mistyped ID here so the operator gets "that is not a Salesforce
	// record ID" instead of a 404 that looks like the account was deleted.
	if err := salesforce.ValidateRecordID(accountID); err != nil {
		return nil, err
	}

	// Field selection is above n8n's parity (its Get returns the whole record
	// and nothing else). Narrowing the response matters on Account, where an
	// org with hundreds of custom fields returns a payload nobody reads.
	fields := salesforce.OptionalString("fields", inputs)
	// A misspelled field name is rejected locally and never reaches Salesforce,
	// so it is a configuration mistake and takes the hard error return. Leaving
	// it to GetRecord would surface it on the soft error port, where it reads
	// as though Salesforce had refused the request.
	for _, f := range salesforce.SplitList(fields) {
		if _, err := salesforce.ValidateSOQLFieldName(f); err != nil {
			return nil, fmt.Errorf("Fields — %w", err)
		}
	}

	record, err := salesforce.GetRecord(instanceURL, token, "Account", accountID, fields)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf("Fetched account %s", accountID)
	if name, ok := record["Name"].(string); ok && name != "" {
		summary = fmt.Sprintf("Fetched account %q (%s)", name, accountID)
	}
	return salesforce.RecordResult(accountID, record, summary), nil
}
