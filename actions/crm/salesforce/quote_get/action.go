package crm_salesforce_quote_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Quote"
	Description  = "Look up one quote by its Salesforce ID and return everything on it - the quote number, its status, what it adds up to, when it expires and who it was sent to."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+magnifying-glass"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "quote_id", Type: core.ConnectionTypeString, Label: "Quote ID", Placeholder: "0Q05f000000AbCdAAK - the quote's Salesforce ID", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "QuoteNumber,Status,GrandTotal,ExpirationDate - leave blank to return every field"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Quote ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Quote"},
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
	// A malformed ID is a wiring mistake, not a Salesforce failure, so it is a
	// hard error — catching it here beats a MALFORMED_ID nobody can act on.
	if err := salesforce.ValidateRecordID(id); err != nil {
		return nil, err
	}

	// Blank fields means "everything readable": Salesforce omits the ?fields=
	// filter and returns the full record, custom fields and computed totals
	// included. On a quote the computed fields (Subtotal, TotalPrice, GrandTotal,
	// LineItemCount) are usually the point of the lookup, so returning the lot is
	// the right default.
	fields := salesforce.OptionalString("fields", inputs)
	// A misspelled field name — the label rather than the API name, which is the
	// commonest Salesforce mistake there is — is rejected locally and never
	// reaches Salesforce, so it takes the hard error return. Left to GetRecord it
	// lands on the soft error port as though Salesforce had refused a well-formed
	// request, and the flow retries a typo forever.
	for _, f := range salesforce.SplitList(fields) {
		if _, err := salesforce.ValidateSOQLFieldName(f); err != nil {
			return nil, fmt.Errorf("Fields — %w", err)
		}
	}

	record, err := salesforce.GetRecord(instanceURL, token, "Quote", id, fields)
	if err != nil {
		if salesforce.ErrorHasCode(err, "INVALID_TYPE") {
			return salesforce.ErrorResult(fmt.Sprintf(
				"quotes are switched off in your Salesforce org — an administrator can turn them on under Setup ▸ Quotes ▸ Quote Settings (%s)", err.Error())), nil
		}
		return salesforce.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf("Retrieved quote %s", id)
	if number, ok := record["QuoteNumber"].(string); ok && number != "" {
		summary = fmt.Sprintf("Retrieved quote %s (%s)", number, id)
	} else if name, ok := record["Name"].(string); ok && name != "" {
		summary = fmt.Sprintf("Retrieved quote %q (%s)", name, id)
	}
	return salesforce.RecordResult(id, record, summary), nil
}
