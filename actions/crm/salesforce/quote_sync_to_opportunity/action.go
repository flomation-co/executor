package crm_salesforce_quote_sync_to_opportunity

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Sync Quote to Opportunity"
	Description  = "Make the deal match the quote the customer actually received. Salesforce copies the quote's product lines and total onto the deal and keeps the two in step from then on, so the forecast shows the quoted figure rather than an old estimate."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+arrow-right-arrow-left"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "quote_id", Type: core.ConnectionTypeString, Label: "Quote ID", Placeholder: "0Q05f000000AbCdAAK - the quote to sync", Required: true},
	{Name: "opportunity_id", Type: core.ConnectionTypeString, Label: "Opportunity (Deal)", Placeholder: "0065f00000AbCdEAAV - leave blank and we use the deal the quote is already on"},
	{Name: "stop_syncing", Type: core.ConnectionTypeBoolean, Label: "Stop syncing instead (leave the deal as it is now)"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Opportunity ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Applied Changes"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	quoteID := salesforce.OptionalString("quote_id", inputs)
	if err := salesforce.ValidateRecordID(quoteID); err != nil {
		return nil, err
	}
	chosenOpp := salesforce.OptionalString("opportunity_id", inputs)
	if chosenOpp != "" {
		if err := salesforce.ValidateRecordID(chosenOpp); err != nil {
			return nil, err
		}
	}

	// Syncing is a field on the OPPORTUNITY (SyncedQuoteId), not on the quote, and
	// Salesforce will only accept a quote that is a child of that opportunity —
	// verified live: pointing an opportunity at someone else's quote is
	// FIELD_INTEGRITY_EXCEPTION "Synced Quote must be a child of the opportunity".
	// Reading the quote first means the operator normally does not have to supply
	// the opportunity at all, and a mismatch is caught here with the real parent
	// named rather than in Salesforce's prose.
	parentOpp, label, err := readQuote(instanceURL, token, quoteID)
	if err != nil {
		if salesforce.ErrorHasCode(err, "INVALID_TYPE") {
			return salesforce.ErrorResult(fmt.Sprintf(
				"quotes are switched off in your Salesforce org — an administrator can turn them on under Setup ▸ Quotes ▸ Quote Settings (%s)", err.Error())), nil
		}
		return salesforce.ErrorResult(err.Error()), nil
	}

	opportunityID := chosenOpp
	if opportunityID == "" {
		opportunityID = parentOpp
	}
	if opportunityID == "" {
		return nil, fmt.Errorf("quote %s is not linked to a deal, so there is nothing to sync it to — set the quote's Opportunity first (Update Quote), or name the deal here", quoteID)
	}
	if chosenOpp != "" && parentOpp != "" && chosenOpp != parentOpp {
		return nil, fmt.Errorf("quote %s belongs to deal %s, not %s — Salesforce only lets a deal sync a quote of its own, so either sync it to %s or move the quote to the other deal first", quoteID, parentOpp, chosenOpp, parentOpp)
	}

	stop := salesforce.OptionalBool("stop_syncing", inputs)

	// Reading the current synced quote makes the two confusing cases honest: a
	// re-run that changes nothing, and a "stop syncing" aimed at a deal that is
	// syncing somebody else's quote.
	currentlySynced, err := opportunitySyncedQuote(instanceURL, token, opportunityID)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	if stop && currentlySynced != "" && currentlySynced != quoteID {
		return nil, fmt.Errorf("deal %s is syncing a different quote (%s), not %s — stopping the sync here would detach the other quote, so check which one you meant", opportunityID, currentlySynced, quoteID)
	}

	body := map[string]interface{}{}
	if stop {
		// An explicit null is what clears the field; omitting it would leave the
		// sync running. Verified live: PATCH {"SyncedQuoteId":null} answers 204 and
		// the deal keeps the figures it had at that moment.
		body["SyncedQuoteId"] = nil
	} else {
		body["SyncedQuoteId"] = quoteID
	}

	if err := salesforce.UpdateRecord(instanceURL, token, "Opportunity", opportunityID, body); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Salesforce answers 204 No Content, so there is no updated record to echo —
	// this is what was applied, plus what it replaced, so a flow can tell a
	// first-time sync from a re-run.
	record := map[string]interface{}{
		"Id":                      opportunityID,
		"SyncedQuoteId":           body["SyncedQuoteId"],
		"QuoteId":                 quoteID,
		"PreviouslySyncedQuoteId": currentlySynced,
	}

	if stop {
		if currentlySynced == "" {
			return salesforce.RecordResult(opportunityID, record, fmt.Sprintf("Deal %s was not syncing a quote, so nothing changed", opportunityID)), nil
		}
		return salesforce.RecordResult(opportunityID, record, fmt.Sprintf("Stopped syncing quote %s to deal %s — the deal keeps the products and total it has now", label, opportunityID)), nil
	}
	summary := fmt.Sprintf("Syncing quote %s to deal %s — the deal's products and total now match the quote", label, opportunityID)
	if currentlySynced == quoteID {
		summary = fmt.Sprintf("Quote %s was already syncing to deal %s — the deal's products and total match the quote", label, opportunityID)
	} else if currentlySynced != "" {
		summary = fmt.Sprintf("Syncing quote %s to deal %s, replacing quote %s — the deal's products and total now match the new quote", label, opportunityID, currentlySynced)
	}
	return salesforce.RecordResult(opportunityID, record, summary), nil
}

// readQuote reads the deal a quote belongs to, plus a label for the summary that
// an operator will recognise (the quote number, which is what appears on the
// document, falling back to the quote's name).
func readQuote(instanceURL, token, quoteID string) (opportunityID, label string, err error) {
	soql, err := salesforce.BuildQuery(
		"Quote",
		"Id,Name,QuoteNumber,OpportunityId",
		[]salesforce.Condition{{Field: "Id", Operator: "=", Value: quoteID}},
		false, "", 1, true,
	)
	if err != nil {
		return "", "", err
	}
	record, err := salesforce.QueryOne(instanceURL, token, soql)
	if err != nil {
		return "", "", err
	}
	if record == nil {
		return "", "", fmt.Errorf("quote %s was not found, or the connected Salesforce user cannot see it", quoteID)
	}
	label = quoteID
	if number, ok := record["QuoteNumber"].(string); ok && number != "" {
		label = number
	} else if name, ok := record["Name"].(string); ok && name != "" {
		label = name
	}
	return salesforce.StringifyID(record["OpportunityId"]), label, nil
}

// opportunitySyncedQuote reads which quote a deal is currently syncing, if any.
func opportunitySyncedQuote(instanceURL, token, opportunityID string) (string, error) {
	soql, err := salesforce.BuildQuery(
		"Opportunity",
		"Id,SyncedQuoteId",
		[]salesforce.Condition{{Field: "Id", Operator: "=", Value: opportunityID}},
		false, "", 1, true,
	)
	if err != nil {
		return "", err
	}
	record, err := salesforce.QueryOne(instanceURL, token, soql)
	if err != nil {
		return "", err
	}
	if record == nil {
		return "", fmt.Errorf("opportunity %s was not found, or the connected Salesforce user cannot see it", opportunityID)
	}
	return salesforce.StringifyID(record["SyncedQuoteId"]), nil
}
