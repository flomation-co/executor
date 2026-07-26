package crm_salesforce_campaign_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Delete Campaign"
	Description  = "Send a Salesforce campaign to the Recycle Bin. Everyone who was signed up to it goes with it, and restoring the campaign from the Recycle Bin within 15 days brings them all back."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+trash"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "campaign_id", Type: core.ConnectionTypeString, Label: "Campaign ID", Placeholder: "701... — copy it from the end of the campaign's web address", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Campaign ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Deleted Campaign"},
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

	if err := salesforce.DeleteRecord(instanceURL, token, "Campaign", campaignID); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Like an update, a delete answers 204 No Content. Echo the ID back so a
	// later step can log or report what was removed rather than receiving an
	// empty result it cannot do anything with.
	deleted := map[string]interface{}{"Id": campaignID, "deleted": true}
	return salesforce.RecordResult(campaignID, deleted, fmt.Sprintf("Deleted campaign %s — it is in the Recycle Bin and can be restored for 15 days", campaignID)), nil
}
