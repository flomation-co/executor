package crm_salesforce_contract_update

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Update Contract"
	Description  = "Change a contract in Salesforce - push the start date, extend the term, record the signatures, hand it to a new owner. Anything you leave blank is left exactly as it was."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+pen"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "contract_id", Type: core.ConnectionTypeString, Label: "Contract ID", Placeholder: "8005f000001AbCdAAK - the contract to change, not its contract number", Required: true},
	{Name: "contract_status", Type: core.ConnectionTypeString, Label: "Status", Placeholder: "Draft, In Approval Process or Activated - note that Activated cannot be undone"},
	{Name: "start_date", Type: core.ConnectionTypeDateTime, Label: "Start Date", Placeholder: "2026-08-01 (the date only) - the end date is worked out from this plus the term"},
	{Name: "contract_term", Type: core.ConnectionTypeInteger, Label: "Contract Term (Months)", Placeholder: "24 - the length of the contract in MONTHS, not days or years"},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Customer (Account)", Placeholder: "0015f00000AbCdEAAV - the account this contract is with"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Owner", Placeholder: "0055f00000AbCdEAAV - the Salesforce user who should own the contract"},
	{Name: "owner_expiration_notice", Type: core.ConnectionTypeString, Label: "Remind The Owner (Days Before The End)", Placeholder: "60 - Salesforce only accepts 15, 30, 45, 60, 90 or 120; anything else is refused"},
	{Name: "pricebook_id", Type: core.ConnectionTypeString, Label: "Price Book", Placeholder: "01s5f000004AbCdAAK - the price list to use for products on this contract"},
	{Name: "company_signed_id", Type: core.ConnectionTypeString, Label: "Signed By (Your Side)", Placeholder: "0055f00000AbCdEAAV - the Salesforce user who signed for your company"},
	{Name: "company_signed_date", Type: core.ConnectionTypeDateTime, Label: "Date You Signed", Placeholder: "2026-07-24 (the date only)"},
	{Name: "customer_signed_id", Type: core.ConnectionTypeString, Label: "Signed By (Customer Contact)", Placeholder: "0035f00000XyZabAAF - the contact who signed for the customer"},
	{Name: "customer_signed_title", Type: core.ConnectionTypeString, Label: "Customer Signatory's Job Title", Placeholder: "Operations Director"},
	{Name: "customer_signed_date", Type: core.ConnectionTypeDateTime, Label: "Date The Customer Signed", Placeholder: "2026-07-25 (the date only)"},
	{Name: "special_terms", Type: core.ConnectionTypeText, Label: "Special Terms", Placeholder: "Anything agreed that differs from your standard terms"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "Background on the contract, visible to everyone on the account"},
	{Name: "billing_street", Type: core.ConnectionTypeText, Label: "Billing Street", Placeholder: "1 Deansgate"},
	{Name: "billing_city", Type: core.ConnectionTypeString, Label: "Billing City", Placeholder: "Manchester"},
	{Name: "billing_state", Type: core.ConnectionTypeString, Label: "Billing County / State", Placeholder: "Leave this blank unless your org's State list has an entry for it - most orgs check it against a fixed list and refuse anything typed by hand"},
	{Name: "billing_postal_code", Type: core.ConnectionTypeString, Label: "Billing Postcode", Placeholder: "M1 1AA"},
	{Name: "billing_country", Type: core.ConnectionTypeString, Label: "Billing Country", Placeholder: "United Kingdom"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "{\"My_Custom_Field__c\":\"value\"} - any other Salesforce field on the contract"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Contract ID"},
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

	id := salesforce.OptionalString("contract_id", inputs)
	if err := salesforce.ValidateRecordID(id); err != nil {
		return nil, fmt.Errorf("Contract ID — %w. A contract's number (00000100) is not its record ID; the record ID starts with 800", err)
	}

	// Every field is optional and every one goes through Set*IfPresent: an update
	// that posted all its blank inputs would clear the operator's data, which on a
	// live contract means wiping the signature dates and the special terms because
	// they only wanted to extend the term.
	body := map[string]interface{}{}
	salesforce.SetIfPresent(body, inputs, "Status", "contract_status")
	// StartDate and both signature dates are Date fields, not Date/Time — a full
	// ISO timestamp from a date-picker upstream is trimmed to the date part.
	salesforce.SetDateIfPresent(body, inputs, "StartDate", "start_date")
	salesforce.SetIntIfPresent(body, inputs, "ContractTerm", "contract_term")
	salesforce.SetIfPresent(body, inputs, "AccountId", "account_id")
	salesforce.SetIfPresent(body, inputs, "OwnerId", "owner_id")
	// OwnerExpirationNotice is a restricted picklist of day counts held as TEXT
	// ("60", not 60), so it goes through the string helper, not the integer one.
	salesforce.SetIfPresent(body, inputs, "OwnerExpirationNotice", "owner_expiration_notice")
	salesforce.SetIfPresent(body, inputs, "Pricebook2Id", "pricebook_id")
	salesforce.SetIfPresent(body, inputs, "CompanySignedId", "company_signed_id")
	salesforce.SetDateIfPresent(body, inputs, "CompanySignedDate", "company_signed_date")
	salesforce.SetIfPresent(body, inputs, "CustomerSignedId", "customer_signed_id")
	salesforce.SetIfPresent(body, inputs, "CustomerSignedTitle", "customer_signed_title")
	salesforce.SetDateIfPresent(body, inputs, "CustomerSignedDate", "customer_signed_date")
	salesforce.SetIfPresent(body, inputs, "SpecialTerms", "special_terms")
	salesforce.SetIfPresent(body, inputs, "Description", "description")
	salesforce.SetIfPresent(body, inputs, "BillingStreet", "billing_street")
	salesforce.SetIfPresent(body, inputs, "BillingCity", "billing_city")
	salesforce.SetIfPresent(body, inputs, "BillingState", "billing_state")
	salesforce.SetIfPresent(body, inputs, "BillingPostalCode", "billing_postal_code")
	salesforce.SetIfPresent(body, inputs, "BillingCountry", "billing_country")

	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("nothing to update — fill in at least one field to change on the contract")
	}

	// EndDate is deliberately not an input: Salesforce derives it from the start
	// date plus the term, less a day, and reports it neither createable nor
	// updateable, so the only way to move a contract's end date is to change the
	// Start Date or the Contract Term. Verified live — pushing the term from 12 to
	// 24 months moved EndDate from 2027-07-31 to 2028-07-31 on its own.

	if err := salesforce.UpdateRecord(instanceURL, token, "Contract", id, body); err != nil {
		return salesforce.ErrorResult(explainUpdateFailure(err)), nil
	}

	// Salesforce answers an update with 204 No Content — there is no updated
	// record to return. Echo back what was actually applied (plus the ID) so the
	// next node has something to work with and the execution view shows what
	// changed. Use the Get Contract action if the full record is needed.
	changed := salesforce.SortedKeys(body)
	record := make(map[string]interface{}, len(body)+1)
	for k, v := range body {
		record[k] = v
	}
	record["Id"] = id

	return salesforce.RecordResult(id, record, fmt.Sprintf("Updated contract %s — changed %s", id, strings.Join(changed, ", "))), nil
}

// explainUpdateFailure translates the three provider answers a contract update
// runs into that common.go cannot decode on its own.
//
// FAILED_ACTIVATION is the one that catches people out. A contract's status only
// moves forward: setting an Activated contract back to Draft is refused with
// "Choose a valid contract status and save your changes" (verified live), which
// reads like a picklist typo and is nothing of the kind.
func explainUpdateFailure(err error) string {
	switch {
	case salesforce.ErrorHasCode(err, "FAILED_ACTIVATION"):
		return fmt.Sprintf("Salesforce refused that status change — a contract's status only moves forward, so an Activated contract cannot be put back to Draft or In Approval Process. Clone it or create a replacement instead (%s)", err.Error())
	case salesforce.ErrorHasCode(err, "INVALID_OR_NULL_FOR_RESTRICTED_PICKLIST"):
		return fmt.Sprintf("Salesforce only accepts values from its own fixed list for that field — Status must be Draft, In Approval Process or Activated, and the owner reminder must be 15, 30, 45, 60, 90 or 120 days (%s)", err.Error())
	case salesforce.ErrorHasCode(err, "INVALID_TYPE"):
		return fmt.Sprintf("contracts are not available in your Salesforce org — an administrator can switch them on under Setup ▸ Contract Settings, and some Salesforce editions do not include them at all (%s)", err.Error())
	default:
		return err.Error()
	}
}
