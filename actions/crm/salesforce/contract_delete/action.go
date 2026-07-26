package crm_salesforce_contract_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Delete Contract"
	Description  = "Send a contract to the Salesforce Recycle Bin, where an administrator can restore it for 15 days. Activated contracts can be deleted too, so use this carefully on anything live."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+trash"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank — taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "contract_id", Type: core.ConnectionTypeString, Label: "Contract ID", Placeholder: "8005f000001AbCdAAK - the contract to delete, not its contract number", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Contract ID"},
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

	id := salesforce.OptionalString("contract_id", inputs)
	if err := salesforce.ValidateRecordID(id); err != nil {
		return nil, fmt.Errorf("Contract ID — %w. A contract's number (00000100) is not its record ID; the record ID starts with 800", err)
	}

	// Deleting is not idempotent in Salesforce: a second run on the same ID comes
	// back ENTITY_IS_DELETED. That is a provider answer, not a wiring mistake, so
	// it lands on the error port as data and a flow can branch on it.
	//
	// Note what is NOT guarded here: an Activated contract deletes perfectly
	// happily (verified live — 204, straight to the Recycle Bin). Salesforce puts
	// no protection on a live contract, so the warning belongs in the description
	// where an operator reads it before wiring the step up.
	if err := salesforce.DeleteRecord(instanceURL, token, "Contract", id); err != nil {
		if salesforce.ErrorHasCode(err, "INVALID_TYPE") {
			return salesforce.ErrorResult(fmt.Sprintf("contracts are not available in your Salesforce org — an administrator can switch them on under Setup ▸ Contract Settings, and some Salesforce editions do not include them at all (%s)", err.Error())), nil
		}
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Salesforce answers 204 No Content, so the ID we were given is the only thing
	// there is to hand downstream — which is exactly what a "contract cancelled,
	// now tidy up the related records" flow needs.
	record := map[string]interface{}{"Id": id, "deleted": true}
	return salesforce.RecordResult(id, record, fmt.Sprintf("Deleted contract %s — it is in the Salesforce Recycle Bin for 15 days", id)), nil
}
