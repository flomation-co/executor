package crm_salesforce_order_activate

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Activate Order"
	Description  = "Activate an order once its product lines are in place, so it counts as a real order rather than a draft. Salesforce stamps who activated it and when, and locks the product lines from further changes."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+circle-check"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank — taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "order_id", Type: core.ConnectionTypeString, Label: "Order ID", Placeholder: "8015f000000AbCdAAK - the draft order to activate", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Order ID"},
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

	id := salesforce.OptionalString("order_id", inputs)
	if err := salesforce.ValidateRecordID(id); err != nil {
		return nil, err
	}

	// Activating is just Status = "Activated", and hiding that is the point: an
	// operator should not have to know the magic word, nor that it is spelled with
	// a d. Reading the order first is worth one extra call because re-activating an
	// order that is ALREADY active answers a bare 204 (verified live) — so without
	// this the summary would claim to have activated something it did not touch,
	// which is exactly the sort of quiet untruth that makes an execution log
	// useless when a flow is being debugged.
	before, number, err := readOrderStatus(instanceURL, token, id)
	if err != nil {
		if salesforce.ErrorHasCode(err, "INVALID_TYPE") {
			return salesforce.ErrorResult(fmt.Sprintf(
				"orders are switched off in your Salesforce org — an administrator can turn them on under Setup ▸ Order Settings ▸ Enable Orders (%s)", err.Error())), nil
		}
		return salesforce.ErrorResult(err.Error()), nil
	}

	label := id
	if number != "" {
		label = number + " (" + id + ")"
	}

	if before == "Activated" {
		record := map[string]interface{}{"Id": id, "Status": before, "PreviousStatus": before, "changed": false}
		return salesforce.RecordResult(id, record, fmt.Sprintf("Order %s was already activated — nothing changed", label)), nil
	}

	if err := salesforce.UpdateRecord(instanceURL, token, "Order", id, map[string]interface{}{"Status": "Activated"}); err != nil {
		// Salesforce's own text here is good ("An order must have at least one
		// product.") but it arrives with no clue about which step comes next, and
		// this is the failure every first-time order flow hits: the order was
		// created and activated in one go, with nothing in between.
		if salesforce.ErrorHasCode(err, "FAILED_ACTIVATION") {
			return salesforce.ErrorResult(fmt.Sprintf(
				"Salesforce would not activate that order — an order needs at least one product line before it can be activated, so add its products first with Add Product to Order (%s)", err.Error())), nil
		}
		if salesforce.ErrorHasCode(err, "INVALID_TYPE") {
			return salesforce.ErrorResult(fmt.Sprintf(
				"orders are switched off in your Salesforce org — an administrator can turn them on under Setup ▸ Order Settings ▸ Enable Orders (%s)", err.Error())), nil
		}
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Salesforce answers 204 No Content, so this is what was applied plus the
	// status it replaced — enough for a flow to branch on, and Get Order will
	// return the stamped ActivatedDate and ActivatedById if they are needed.
	record := map[string]interface{}{"Id": id, "Status": "Activated", "PreviousStatus": before, "changed": true}
	return salesforce.RecordResult(id, record, fmt.Sprintf("Activated order %s — its product lines are now locked, so put it back to Draft (Update Order) if they need changing", label)), nil
}

// readOrderStatus reads an order's current status and its order number, so the
// action can tell an activation from a no-op and name the order the way the
// operator sees it in Salesforce.
func readOrderStatus(instanceURL, token, orderID string) (status, number string, err error) {
	soql, err := salesforce.BuildQuery(
		"Order",
		"Id,OrderNumber,Status",
		[]salesforce.Condition{{Field: "Id", Operator: "=", Value: orderID}},
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
		return "", "", fmt.Errorf("order %s was not found, or the connected Salesforce user cannot see it", orderID)
	}
	status, _ = record["Status"].(string)
	number, _ = record["OrderNumber"].(string)
	return status, number, nil
}
