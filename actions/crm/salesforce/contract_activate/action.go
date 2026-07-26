package crm_salesforce_contract_activate

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Activate Contract"
	Description  = "Mark a signed contract as Activated so it counts as live. Salesforce stamps who activated it and when. Activating is one-way, so this is the step to run once the signature is actually in."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+circle-check"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

// activatedStatus is the standard Salesforce contract status that means "live".
//
// It is the value Salesforce ships and the one an org keeps unless somebody has
// deliberately renamed it, which is why the Status input exists as an override
// rather than as a required field an operator has to fill in every time.
const activatedStatus = "Activated"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "contract_id", Type: core.ConnectionTypeString, Label: "Contract ID", Placeholder: "8005f000001AbCdAAK - the contract to activate, not its contract number", Required: true},
	{Name: "contract_status", Type: core.ConnectionTypeString, Label: "Status To Set", Placeholder: "Leave blank to use Activated - only change this if your administrator has renamed the live status in your org"},
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

	id := salesforce.OptionalString("contract_id", inputs)
	if err := salesforce.ValidateRecordID(id); err != nil {
		return nil, fmt.Errorf("Contract ID — %w. A contract's number (00000100) is not its record ID; the record ID starts with 800", err)
	}

	// There is no "activate" endpoint: activation is a status change, and
	// Salesforce stamps ActivatedById and ActivatedDate itself when it lands
	// (verified live). Setting those by hand is neither necessary nor allowed on
	// create, so this action sends the one field.
	status := salesforce.OptionalString("contract_status", inputs)
	if status == "" {
		status = activatedStatus
	}

	if err := salesforce.UpdateRecord(instanceURL, token, "Contract", id, map[string]interface{}{"Status": status}); err != nil {
		return salesforce.ErrorResult(explainActivationFailure(err, status)), nil
	}

	// Re-read the contract, best effort, for two reasons worth the one extra call.
	//
	// The first is that an update answers 204 with no body, so without it the only
	// thing to report is the ID that was passed in. The second is the real one: a
	// contract activated with no start date or no term has NO end date, because
	// Salesforce derives EndDate from those two and cannot be given one directly —
	// and it activates the contract perfectly happily anyway (verified live: 204,
	// ActivatedDate stamped, EndDate null). Every renewals flow then silently skips
	// it forever. That is worth saying out loud at the moment it happens.
	record, ok := readContract(instanceURL, token, id)
	if !ok {
		return salesforce.RecordResult(id, map[string]interface{}{"Id": id, "Status": status}, fmt.Sprintf("Activated contract %s — its status is now %q", id, status)), nil
	}

	number := stringField(record, "ContractNumber")
	label := id
	if number != "" {
		label = fmt.Sprintf("%s (%s)", number, id)
	}
	summary := fmt.Sprintf("Activated contract %s — its status is now %q, but it has NO end date because its start date or term is blank, so renewal reminders will never pick it up. Set the Start Date and Contract Term to fix that", label, status)
	if endDate := stringField(record, "EndDate"); endDate != "" {
		summary = fmt.Sprintf("Activated contract %s — its status is now %q and it runs to %s", label, status, endDate)
	}
	return salesforce.RecordResult(id, record, summary), nil
}

// contractStateFields is the projection the read-back needs: enough to report
// what happened and to spot the missing-end-date trap.
const contractStateFields = "Id,ContractNumber,Status,StatusCode,StartDate,EndDate,ContractTerm,ActivatedById,ActivatedDate,AccountId,OwnerId"

// readContract re-reads the contract after the status change. A failure here is
// NOT fatal — the activation already succeeded, and reporting it as an error
// would send a flow down its failure branch over a record it just changed
// correctly. The caller falls back to the minimal summary instead.
func readContract(instanceURL, token, id string) (map[string]interface{}, bool) {
	record, err := salesforce.GetRecord(instanceURL, token, "Contract", id, contractStateFields)
	if err != nil || len(record) == 0 {
		return nil, false
	}
	return record, true
}

// stringField reads a string field from a record, tolerating the JSON null
// Salesforce sends for an empty date.
func stringField(record map[string]interface{}, field string) string {
	v, ok := record[field].(string)
	if !ok {
		return ""
	}
	return v
}

// explainActivationFailure translates the answers an activation actually gets
// back into something an operator can act on.
//
// FAILED_ACTIVATION is Salesforce's catch-all for "that status transition is not
// allowed", and its own text — "Choose a valid contract status and save your
// changes" — reads like a spelling mistake. Verified live, the commonest cause is
// the opposite: the contract is ALREADY Activated and something is trying to move
// it back. Re-activating an Activated contract is a no-op that succeeds, so if
// this code comes back the transition really was refused.
func explainActivationFailure(err error, status string) string {
	switch {
	case salesforce.ErrorHasCode(err, "FAILED_ACTIVATION"):
		return fmt.Sprintf("Salesforce refused to move this contract to %q — a contract's status only moves forward, and your org may also require an approval before activation. Check the contract's current status in Salesforce (%s)", status, err.Error())
	case salesforce.ErrorHasCode(err, "INVALID_OR_NULL_FOR_RESTRICTED_PICKLIST"):
		return fmt.Sprintf("%q is not one of your org's contract statuses — the standard list is Draft, In Approval Process and Activated. Leave Status To Set blank to use Activated (%s)", status, err.Error())
	case salesforce.ErrorHasCode(err, "INVALID_TYPE"):
		return fmt.Sprintf("contracts are not available in your Salesforce org — an administrator can switch them on under Setup ▸ Contract Settings, and some Salesforce editions do not include them at all (%s)", err.Error())
	default:
		return err.Error()
	}
}
