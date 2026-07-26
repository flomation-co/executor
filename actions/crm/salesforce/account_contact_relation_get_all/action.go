package crm_salesforce_account_contact_relation_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Many Account-Contact Relationships"
	Description  = "List who is connected to a company and in what role — or, given a contact, every company they are connected to. Covers the related contacts a company has as well as its own direct ones."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+user-group"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

// defaultRelationFields is the SELECT list used when the operator picks none.
//
// The shared DefaultFields helper falls back to Id,Name,LastModifiedDate for an
// object it does not know, and AccountContactRelation has no Name field at all
// — that query would fail with INVALID_FIELD. This list is also chosen to be
// readable on its own: the two related records are pulled in by name so the
// output does not consist entirely of record IDs.
const defaultRelationFields = "Id, AccountId, Account.Name, ContactId, Contact.Name, Roles, IsActive, IsDirect, StartDate, EndDate, CreatedDate"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Account", Placeholder: "Record ID of the company, to list its contacts, e.g. 0015f00000AbCdEAAV"},
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact", Placeholder: "Record ID of the contact, to list their companies, e.g. 0035f00000XyZabAAF"},
	{Name: "active_only", Type: core.ConnectionTypeBoolean, Label: "Active Relationships Only"},
	{Name: "role", Type: core.ConnectionTypeString, Label: "Role", Placeholder: "Decision Maker (returns relationships that include this role)"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All (fetch every match)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 (max 2000); ignored when Return All is on"},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Id, Contact.Name, Roles (blank returns a readable default set)"},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Sort By", Placeholder: "Contact.Name ASC, or CreatedDate DESC"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Relationships"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total_size", Type: core.ConnectionTypeInteger, Label: "Records Returned"},
	{Name: "next_url", Type: core.ConnectionTypeString, Label: "Next Page URL"},
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

	accountID := salesforce.OptionalString("account_id", inputs)
	contactID := salesforce.OptionalString("contact_id", inputs)
	// Either side identifies the relationship, but neither would list every
	// relationship in the org — a query that big is never what the operator
	// meant and would burn the org's API allowance.
	if accountID == "" && contactID == "" {
		return nil, fmt.Errorf("set either account_id (to list a company's contacts) or contact_id (to list a contact's companies)")
	}

	conditions := make([]salesforce.Condition, 0, 4)
	if accountID != "" {
		if err := salesforce.ValidateRecordID(accountID); err != nil {
			return nil, fmt.Errorf("Account — %w", err)
		}
		conditions = append(conditions, salesforce.Condition{Field: "AccountId", Operator: "=", Value: accountID})
	}
	if contactID != "" {
		if err := salesforce.ValidateRecordID(contactID); err != nil {
			return nil, fmt.Errorf("Contact — %w", err)
		}
		conditions = append(conditions, salesforce.Condition{Field: "ContactId", Operator: "=", Value: contactID})
	}
	if salesforce.OptionalBool("active_only", inputs) {
		conditions = append(conditions, salesforce.Condition{Field: "IsActive", Operator: "=", Value: "true"})
	}
	if role := salesforce.OptionalString("role", inputs); role != "" {
		// Roles is a multi-select picklist, so an equality test would only
		// match a relationship holding that ONE role. INCLUDES is the operator
		// SOQL provides for "has this among its values".
		conditions = append(conditions, salesforce.Condition{Field: "Roles", Operator: "INCLUDES", Value: role})
	}

	fields := salesforce.OptionalString("fields", inputs)
	if fields == "" {
		fields = defaultRelationFields
	}

	returnAll := salesforce.OptionalBool("return_all", inputs)
	limit, limitSet := salesforce.OptionalInt("limit", inputs)
	// BuildQueryTyped, not BuildQuery: the Role and Active filters are rendered
	// according to the real Salesforce type of Roles (a multi-select picklist,
	// quoted) and IsActive (a tick-box, bare), rather than guessed from the
	// value. A describe the connected user cannot run degrades to the heuristic
	// rather than failing the action.
	soql, err := salesforce.BuildQueryTyped(
		instanceURL,
		token,
		"AccountContactRelation",
		fields,
		conditions,
		false,
		salesforce.OptionalString("order_by", inputs),
		salesforce.ClampLimit(limit, limitSet),
		!returnAll,
	)
	if err != nil {
		return nil, err
	}

	records, nextURL, totalSize, pages, err := salesforce.Query(instanceURL, token, soql, returnAll, false)
	if err != nil {
		// An org without Contacts to Multiple Accounts has no such object;
		// CheckResponse turns that INVALID_TYPE into a readable explanation.
		return salesforce.ErrorResult(err.Error()), nil
	}

	subject := fmt.Sprintf("account %s", accountID)
	if accountID == "" {
		subject = fmt.Sprintf("contact %s", contactID)
	}
	summary := fmt.Sprintf("Found %d relationship(s) for %s", len(records), subject)
	switch {
	case returnAll && nextURL != "" && pages >= salesforce.MaxAllPages:
		summary = fmt.Sprintf("Fetched %d relationship(s) for %s across %d page(s); stopped at the %d-page safety cap", len(records), subject, pages, salesforce.MaxAllPages)
	case returnAll:
		summary = fmt.Sprintf("Fetched all %d relationship(s) for %s", len(records), subject)
	case nextURL != "":
		summary = fmt.Sprintf("Found %d relationship(s) of %d for %s — turn on Return All to fetch the rest", len(records), totalSize, subject)
	}
	return salesforce.ListResult(records, nextURL, totalSize, summary), nil
}
