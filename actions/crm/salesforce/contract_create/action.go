package crm_salesforce_contract_create

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Create Contract"
	Description  = "Set up a contract against a customer's account - when it starts, how many months it runs for and who signed it. Salesforce gives it its own contract number and works the end date out from the term, so this is the record to hang the signed PDF off. New contracts always start as a draft; add an Activate Contract step to make one live."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+plus"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Customer (Account)", Placeholder: "0015f00000AbCdEAAV - the account this contract is with; Salesforce will not create a contract without one", Required: true},
	{Name: "contract_status", Type: core.ConnectionTypeString, Label: "Status", Placeholder: "Leave blank - a new contract always starts as Draft. Move it on afterwards with Update Contract, or make it live with Activate Contract"},
	{Name: "start_date", Type: core.ConnectionTypeDateTime, Label: "Start Date", Placeholder: "2026-08-01 (the date only) - the end date is worked out from this plus the term"},
	{Name: "contract_term", Type: core.ConnectionTypeInteger, Label: "Contract Term (Months)", Placeholder: "12 - the length of the contract in MONTHS, not days or years"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Owner", Placeholder: "0055f00000AbCdEAAV - leave blank and the connected Salesforce user owns it"},
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
	{Name: "record_type_id", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "0125f000000AbCdAAK - only if your org uses record types on contracts"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "{\"My_Custom_Field__c\":\"value\"} - any other Salesforce field on the contract"},
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

	// AccountId is the ONE field Salesforce insists on — verified live: a Contract
	// posted with nothing but an account is created, numbered and left in Draft.
	// Status and Owner are both defaultedOnCreate, so leaving them blank is
	// correct rather than incomplete.
	accountID := salesforce.OptionalString("account_id", inputs)
	if accountID == "" {
		return nil, fmt.Errorf("choose the customer this contract is with — Salesforce will not create a contract without an account, e.g. 0015f00000AbCdEAAV")
	}
	if err := salesforce.ValidateRecordID(accountID); err != nil {
		return nil, fmt.Errorf("Customer (Account) — %w", err)
	}

	body := map[string]interface{}{"AccountId": accountID}

	// A new contract can only be a DRAFT. Salesforce's insert rule is that Status
	// has to map to StatusCode "Draft", and only Draft does — verified live, both
	// In Approval Process and Activated are refused outright with FAILED_ACTIVATION
	// ("Choose a valid contract status and save your changes"), which reads like a
	// spelling mistake and is nothing of the kind. Both values are offered by the
	// live Status dropdown on this input, with In Approval Process FIRST, so
	// two-thirds of what an operator is shown here cannot work; say so before the
	// call rather than translating the refusal afterwards.
	//
	// Anything else is passed through untouched: an org can add its own values to
	// the Contract Status list, and one mapped to StatusCode Draft is genuinely
	// creatable. Refusing a status this action cannot know about would be worse
	// than letting Salesforce judge it.
	status := salesforce.OptionalString("contract_status", inputs)
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "activated":
		return nil, fmt.Errorf("a contract cannot be created as Activated — Salesforce only activates a contract that already exists. Leave Status blank to create it as a Draft, then add an Activate Contract step after this one")
	case "in approval process":
		return nil, fmt.Errorf("a contract cannot be created as In Approval Process — Salesforce insists a new contract starts as a Draft. Leave Status blank here, then move it to In Approval Process with Update Contract once it exists")
	}
	salesforce.SetIfPresent(body, inputs, "Status", "contract_status")
	// StartDate, and both signature dates, are Date fields rather than Date/Time.
	// A date-picker upstream hands over a full ISO timestamp, so trim it.
	salesforce.SetDateIfPresent(body, inputs, "StartDate", "start_date")
	salesforce.SetIntIfPresent(body, inputs, "ContractTerm", "contract_term")
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
	salesforce.SetIfPresent(body, inputs, "RecordTypeId", "record_type_id")

	// Every org has custom fields on Contract — renewal owner, notice period,
	// signed-PDF link — and none of them can be a first-class input here.
	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}

	id, raw, err := salesforce.CreateRecord(instanceURL, token, "Contract", body)
	if err != nil {
		return salesforce.ErrorResult(explainCreateFailure(err, status)), nil
	}

	summary := fmt.Sprintf("Created contract %s for account %s — Salesforce has given it its own contract number", id, accountID)
	if term, ok := salesforce.OptionalInt("contract_term", inputs); ok && term > 0 {
		summary = fmt.Sprintf("Created contract %s for account %s, running for %d month(s) — Salesforce has given it its own contract number and works the end date out from the start date and the term", id, accountID, term)
	}
	return salesforce.RecordResult(id, raw, summary), nil
}

// explainCreateFailure names the three provider answers an operator cannot
// decode on their own.
//
// FAILED_ACTIVATION is the one that would otherwise waste an afternoon. A
// Contract CANNOT be created already Activated — verified live, posting
// Status:"Activated" on insert is refused outright with "Choose a valid contract
// status and save your changes", which sounds exactly like a spelling mistake and
// is nothing of the kind. Activation is a second step, and Salesforce is
// deliberate about that: it is what makes ActivatedBy and ActivatedDate mean
// something. Salesforce accepts the same value happily on a PATCH.
//
// The picklist one matters nearly as much. Contract Status and Owner Expiration
// Notice are both RESTRICTED picklists in a standard org (verified live: Draft /
// In Approval Process / Activated, and 15 / 30 / 45 / 60 / 90 / 120), and
// Salesforce answers a value outside those lists with
// INVALID_OR_NULL_FOR_RESTRICTED_PICKLIST — a code common.go does not translate,
// because nothing in v1 wrote to a restricted picklist.
func explainCreateFailure(err error, status string) string {
	switch {
	case salesforce.ErrorHasCode(err, "FAILED_ACTIVATION"):
		// Name the status that was actually sent. The old wording only ever said
		// "Activated", so an operator who had set a custom status of their own read
		// advice about a value that was nowhere on their screen.
		if status != "" && !strings.EqualFold(strings.TrimSpace(status), "activated") {
			return fmt.Sprintf("Salesforce will not create a contract with Status %q — a new contract has to start as a Draft, whatever your org's status list offers. Leave Status blank here, then set the status you want with Update Contract, or make it live with Activate Contract (%s)", status, err.Error())
		}
		return fmt.Sprintf("a contract cannot be created as Activated — Salesforce only lets a contract be activated after it exists. Leave Status blank (or set Draft) here, then add an Activate Contract step after this one (%s)", err.Error())
	case salesforce.ErrorHasCode(err, "INVALID_OR_NULL_FOR_RESTRICTED_PICKLIST"):
		return fmt.Sprintf("Salesforce only accepts values from its own fixed list for that field — the standard contract statuses are Draft, In Approval Process and Activated (and only Draft can be used when creating one), and the owner reminder must be 15, 30, 45, 60, 90 or 120 days (%s)", err.Error())
	case salesforce.ErrorHasCode(err, "INVALID_TYPE"):
		return fmt.Sprintf("contracts are not available in your Salesforce org — an administrator can switch them on under Setup ▸ Contract Settings, and some Salesforce editions do not include them at all (%s)", err.Error())
	default:
		return err.Error()
	}
}
