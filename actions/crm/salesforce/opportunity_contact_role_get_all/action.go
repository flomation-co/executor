package crm_salesforce_opportunity_contact_role_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Opportunity Contacts"
	Description  = "List everyone involved in a deal and the part each of them plays, so a flow can email the decision maker or check a deal actually has one."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+user-group"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

// defaultFields is the projection used when the operator picks no fields.
//
// Contact.Name and Contact.Email are relationship traversals rather than fields
// on the junction record itself. Without them the result is a list of IDs, which
// tells a front-of-house operator nothing — they want the person's name.
const defaultFields = "Id,OpportunityId,ContactId,Contact.Name,Contact.Email,Contact.Phone,Role,IsPrimary"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "opportunity_id", Type: core.ConnectionTypeString, Label: "Opportunity ID", Placeholder: "0065f00000AbCdEAAV - the deal whose contacts you want", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Role,Contact.Name,Contact.Email - leave blank for names, emails and roles"},
	{Name: "primary_only", Type: core.ConnectionTypeBoolean, Label: "Only The Primary Contact"},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Sort By", Placeholder: "Role ASC - or IsPrimary DESC to put the primary contact first"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All (fetch every contact)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 (max 2000); ignored when Return All is on"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Contact Roles"},
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

	opportunityID := salesforce.OptionalString("opportunity_id", inputs)
	if err := salesforce.ValidateRecordID(opportunityID); err != nil {
		return nil, err
	}

	fields := salesforce.OptionalString("fields", inputs)
	if fields == "" {
		fields = defaultFields
	}

	conditions := []salesforce.Condition{{Field: "OpportunityId", Operator: "=", Value: opportunityID}}
	// "Who do I actually write to?" is the common question, and the primary
	// contact is the answer — so it gets a checkbox rather than a filter the
	// operator has to compose.
	if salesforce.OptionalBool("primary_only", inputs) {
		conditions = append(conditions, salesforce.Condition{Field: "IsPrimary", Operator: "=", Value: "true"})
	}

	returnAll := salesforce.OptionalBool("return_all", inputs)
	limit, limitSet := salesforce.OptionalInt("limit", inputs)

	soql, err := salesforce.BuildQuery(
		"OpportunityContactRole",
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
		return salesforce.ErrorResult(err.Error()), nil
	}

	out := salesforce.ListResult(records, nextURL, totalSize, "")
	if returnAll && nextURL != "" && pages >= salesforce.MaxAllPages {
		out["tool_result"] = fmt.Sprintf("Fetched %d contact role(s) on opportunity %s across %d page(s); stopped at the %d-page safety cap", len(records), opportunityID, pages, salesforce.MaxAllPages)
	} else {
		out["tool_result"] = fmt.Sprintf("Found %d contact role(s) on opportunity %s", len(records), opportunityID)
	}
	return out, nil
}
