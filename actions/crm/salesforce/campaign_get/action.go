package crm_salesforce_campaign_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Campaign"
	Description  = "Look up one Salesforce campaign and everything it has recorded — its dates, budget and the running totals of how many leads, contacts, responses and opportunities it has produced."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+magnifying-glass"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank — taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "campaign_id", Type: core.ConnectionTypeString, Label: "Campaign ID", Placeholder: "701... — copy it from the end of the campaign's web address", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Leave blank for every field, or list them: Name, Status, NumberOfResponses, ActualCost"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Campaign ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Campaign"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	campaignID := salesforce.OptionalString("campaign_id", inputs)
	if err := salesforce.ValidateRecordID(campaignID); err != nil {
		return nil, err
	}

	// GetRecord validates the field list too, but doing it here first keeps the
	// error discipline honest: a typo in a field name is a configuration
	// mistake and must fail the step hard, not arrive as data on the error port
	// looking like Salesforce was unavailable.
	fields := salesforce.OptionalString("fields", inputs)
	for _, f := range salesforce.SplitList(fields) {
		if _, err := salesforce.ValidateSOQLFieldName(f); err != nil {
			return nil, err
		}
	}

	record, err := salesforce.GetRecord(instanceURL, token, "Campaign", campaignID, fields)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Name is only in the response when the operator asked for it (or asked for
	// everything), so fall back to the ID for the summary line.
	label := campaignID
	if n, ok := record["Name"].(string); ok && n != "" {
		label = n
	}
	return salesforce.RecordResult(campaignID, record, fmt.Sprintf("Fetched campaign %q (%s)", label, campaignID)), nil
}
