package crm_salesforce_contract_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Contract"
	Description  = "Look up one contract by its Salesforce ID and return everything on it - its contract number, status, start date, term, the end date Salesforce works out for you, who signed it and every custom field."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+magnifying-glass"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "contract_id", Type: core.ConnectionTypeString, Label: "Contract ID", Placeholder: "8005f000001AbCdAAK - the contract's Salesforce ID, not its contract number", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "ContractNumber,Status,StartDate,EndDate,ContractTerm - leave blank to return every field"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Contract ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Contract"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	// A malformed ID is a wiring mistake, not a Salesforce failure, so it is a
	// hard error — catching it here beats a MALFORMED_ID nobody can act on.
	// Worth naming the trap: a contract's visible identifier is its
	// ContractNumber ("00000100"), which is NOT its record ID, and pasting the
	// number is the obvious mistake to make.
	id := salesforce.OptionalString("contract_id", inputs)
	if err := salesforce.ValidateRecordID(id); err != nil {
		return nil, fmt.Errorf("Contract ID — %w. A contract's number (00000100) is not its record ID; the record ID starts with 800", err)
	}

	// Blank fields means "everything readable": Salesforce omits the ?fields=
	// filter and returns the full sObject, custom fields included. On a contract
	// that is usually what is wanted, because the renewal date or notice period a
	// flow needs is very often a custom field nobody thought to list.
	fields := salesforce.OptionalString("fields", inputs)
	// A misspelled field name — the label rather than the API name, which is the
	// commonest Salesforce mistake there is — is rejected locally and never
	// reaches Salesforce, so it is a configuration mistake and takes the hard
	// error return. Left to GetRecord it lands on the soft error port as though
	// Salesforce had refused a well-formed request, and the flow carries on down
	// its failure branch retrying a typo forever.
	for _, f := range salesforce.SplitList(fields) {
		if _, err := salesforce.ValidateSOQLFieldName(f); err != nil {
			return nil, fmt.Errorf("Fields — %w", err)
		}
	}

	record, err := salesforce.GetRecord(instanceURL, token, "Contract", id, fields)
	if err != nil {
		if salesforce.ErrorHasCode(err, "INVALID_TYPE") {
			return salesforce.ErrorResult(fmt.Sprintf("contracts are not available in your Salesforce org — an administrator can switch them on under Setup ▸ Contract Settings, and some Salesforce editions do not include them at all (%s)", err.Error())), nil
		}
		return salesforce.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf("Retrieved contract %s", id)
	if number, ok := record["ContractNumber"].(string); ok && number != "" {
		summary = fmt.Sprintf("Retrieved contract %s (%s)", number, id)
	}
	return salesforce.RecordResult(id, record, summary), nil
}
