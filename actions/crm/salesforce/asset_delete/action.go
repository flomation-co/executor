package crm_salesforce_asset_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Delete Asset"
	Description  = "Send an asset to the Salesforce Recycle Bin, where an administrator can restore it for 15 days. If a unit has simply been retired rather than recorded by mistake, set its status to Obsolete instead so the customer's history is kept. Deleting an asset that other assets sit under empties their Part Of link without warning."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+trash"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank — taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "asset_id", Type: core.ConnectionTypeString, Label: "Asset ID", Placeholder: "02i5f000000AbCdAAK - the asset to delete, not its serial number", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Asset ID"},
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

	id := salesforce.OptionalString("asset_id", inputs)
	if err := salesforce.ValidateRecordID(id); err != nil {
		return nil, fmt.Errorf("Asset ID — %w. A serial number is not a record ID; an asset's record ID starts with 02i", err)
	}

	// Deleting is not idempotent in Salesforce: a second run on the same ID comes
	// back ENTITY_IS_DELETED. That is a provider answer, not a wiring mistake, so
	// it lands on the error port as data and a flow can branch on it.
	//
	// Salesforce does NOT protect a parent asset from being deleted. Verified live:
	// deleting an asset that a child asset names as its "Part Of" succeeds with
	// 204, and the child is left behind with its Part Of link silently emptied. So
	// there is nothing to guard against here and no error to translate — the
	// warning belongs in the description, where an operator reads it before wiring
	// the step up.
	if err := salesforce.DeleteRecord(instanceURL, token, "Asset", id); err != nil {
		if salesforce.ErrorHasCode(err, "INVALID_TYPE") {
			return salesforce.ErrorResult(fmt.Sprintf("assets are not available in your Salesforce org — an administrator can switch the Assets tab and object permissions on, and some Salesforce editions do not include them at all (%s)", err.Error())), nil
		}
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Salesforce answers 204 No Content, so the ID we were given is the only thing
	// there is to hand downstream — which is exactly what a "kit returned, now tidy
	// up the related records" flow needs.
	record := map[string]interface{}{"Id": id, "deleted": true}
	return salesforce.RecordResult(id, record, fmt.Sprintf("Deleted asset %s — it is in the Salesforce Recycle Bin for 15 days", id)), nil
}
