package crm_salesforce_order_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Delete Order"
	Description  = "Send an order and its product lines to the Salesforce Recycle Bin, where an administrator can restore it for 15 days. Only draft orders can be deleted - an activated one has to be put back to Draft first."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+trash"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "order_id", Type: core.ConnectionTypeString, Label: "Order ID", Placeholder: "8015f000000AbCdAAK - the order to delete", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Order ID"},
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

	id := salesforce.OptionalString("order_id", inputs)
	if err := salesforce.ValidateRecordID(id); err != nil {
		return nil, err
	}

	// Deleting is not idempotent in Salesforce: a second run on the same ID comes
	// back 404 ENTITY_IS_DELETED. That is a provider answer, not a wiring mistake,
	// so it lands on the error port as data and a flow can branch on it.
	if err := salesforce.DeleteRecord(instanceURL, token, "Order", id); err != nil {
		// Verified live: deleting an ACTIVATED order is DELETE_FAILED "unable to
		// modify activated order", and the fix is not obvious — you set the order
		// back to Draft (which Salesforce does allow) and then delete it. Without
		// that sentence an operator reasonably concludes activated orders can never
		// be removed.
		if salesforce.ErrorHasCode(err, "DELETE_FAILED") || salesforce.ErrorHasCode(err, "ENTITY_IS_LOCKED") {
			return salesforce.ErrorResult(fmt.Sprintf(
				"that order is activated, so Salesforce will not delete it — set its Status back to Draft with Update Order first, then delete it (%s)", err.Error())), nil
		}
		if salesforce.ErrorHasCode(err, "INVALID_TYPE") {
			return salesforce.ErrorResult(fmt.Sprintf(
				"orders are switched off in your Salesforce org — an administrator can turn them on under Setup ▸ Order Settings ▸ Enable Orders (%s)", err.Error())), nil
		}
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Salesforce answers 204 No Content, so the ID we were given is the only thing
	// there is to hand downstream — which is what a "order cancelled, now tidy up"
	// flow needs. Deleting an order takes its product lines with it.
	record := map[string]interface{}{"Id": id, "deleted": true}
	return salesforce.RecordResult(id, record, fmt.Sprintf("Deleted order %s and its product lines — it is in the Salesforce Recycle Bin for 15 days", id)), nil
}
