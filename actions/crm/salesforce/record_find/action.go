package crm_salesforce_record_find

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Find Record by Field"
	Description  = "Look up a single record by one of its values — an email address, an order number, a reference from another system. If nothing matches, the action still succeeds with an empty Record ID, so your flow can go on to create the record instead."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+key"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "object", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "Contact, Account, Lead, Invoice__c", Required: true},
	{Name: "match_field", Type: core.ConnectionTypeString, Label: "Look Up By", Placeholder: "Email", Required: true},
	{Name: "match_value", Type: core.ConnectionTypeString, Label: "Value to Match", Placeholder: "jane@example.com", Required: true},
	{
		Name:        "match_type",
		Type:        core.ConnectionTypeString,
		Label:       "How to Match",
		Placeholder: "Exactly",
		Options: []core.ConnectionOption{
			{Name: "Exactly", Value: "exactly"},
			{Name: "Contains", Value: "contains"},
			{Name: "Starts With", Value: "starts_with"},
			{Name: "Ends With", Value: "ends_with"},
		},
	},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Details to Return", Placeholder: "Id, FirstName, LastName, Email — leave blank for the usual details"},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "When Several Match, Prefer", Placeholder: "CreatedDate DESC — decides which record wins"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Record ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Record"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	object, err := salesforce.RequiredString("object", inputs)
	if err != nil {
		return nil, err
	}
	matchField, err := salesforce.RequiredString("match_field", inputs)
	if err != nil {
		return nil, err
	}
	matchValue, err := salesforce.RequiredString("match_value", inputs)
	if err != nil {
		return nil, err
	}

	condition, err := buildCondition(matchField, matchValue, salesforce.OptionalString("match_type", inputs))
	if err != nil {
		return nil, err
	}

	// Fetch two rather than one. The extra row costs nothing (Salesforce
	// returns a single page either way) and it is the only way to tell "this
	// value identifies one record" from "this value is ambiguous" — which is
	// the difference between a lookup an operator can trust and one that
	// silently picks a record at random.
	//
	// Typed, because the whole point of this action is looking a record up by
	// whatever value the operator has to hand — an order number, an account
	// number, a badge ID. Whether that literal needs quoting depends on the
	// field, not on how the value looks, and guessing wrong is INVALID_FIELD.
	soql, err := salesforce.BuildQueryTyped(
		instanceURL, token,
		object,
		withID(defaultedFields(instanceURL, token, object, salesforce.OptionalString("fields", inputs))),
		[]salesforce.Condition{condition},
		false,
		salesforce.OptionalString("order_by", inputs),
		2,
		true,
	)
	if err != nil {
		// Every failure BuildQuery can raise is a bad object name, field name,
		// operator or sort direction — all configuration mistakes.
		return nil, err
	}

	records, _, _, _, err := salesforce.Query(instanceURL, token, soql, false, false)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	if len(records) == 0 {
		// A lookup that finds nothing is a normal outcome, not a failure: the
		// canonical flow is "find the contact, and if there isn't one, create
		// it". Returning an empty Record ID on the success port is what lets an
		// operator branch on it without a dead error branch.
		return salesforce.RecordResult("", nil, fmt.Sprintf("No %s found where %s %s", object, matchField, describeMatch(matchValue, salesforce.OptionalString("match_type", inputs)))), nil
	}

	record := records[0]
	id := salesforce.StringifyID(record["Id"])
	summary := fmt.Sprintf("Found %s %s", object, matchDescription(record, id, matchField, matchValue))
	if len(records) > 1 {
		summary += " — more than one record matched, so the first was used; set When Several Match, or use a value that is unique"
	}
	return salesforce.RecordResult(id, record, summary), nil
}

// buildCondition turns the operator's match choice into a validated WHERE term.
//
// Everything goes through salesforce.Condition rather than a hand-built clause:
// BuildWhere is the injection boundary, and the wildcard forms below only ever
// add % to a value that BuildWhere then escapes and quotes. A % the operator
// typed themselves stays a wildcard, which is the behaviour they get from
// Salesforce's own search box.
func buildCondition(field, value, matchType string) (salesforce.Condition, error) {
	switch strings.ToLower(strings.TrimSpace(matchType)) {
	case "", "exactly":
		return salesforce.Condition{Field: field, Operator: "=", Value: value}, nil
	case "contains":
		return salesforce.Condition{Field: field, Operator: "LIKE", Value: "%" + value + "%"}, nil
	case "starts_with":
		return salesforce.Condition{Field: field, Operator: "LIKE", Value: value + "%"}, nil
	case "ends_with":
		return salesforce.Condition{Field: field, Operator: "LIKE", Value: "%" + value}, nil
	}
	return salesforce.Condition{}, fmt.Errorf("%q is not a way of matching — choose Exactly, Contains, Starts With or Ends With", matchType)
}

// withID makes sure Id is part of the SELECT list. The Record ID is this
// action's headline output — it is what the next node in the flow chains off —
// and an operator who narrows the fields down to "Email" would otherwise get a
// blank one back with nothing to explain why. A blank list needs no help: the
// per-object defaults already start with Id.
func withID(fields string) string {
	if strings.TrimSpace(fields) == "" {
		return ""
	}
	for _, f := range salesforce.SplitList(fields) {
		if strings.EqualFold(f, "Id") {
			return fields
		}
	}
	return "Id," + fields
}

// describeMatch renders the comparison the way the operator set it up, so a
// not-found message says what was actually looked for.
func describeMatch(value, matchType string) string {
	switch strings.ToLower(strings.TrimSpace(matchType)) {
	case "contains":
		return fmt.Sprintf("contains %q", value)
	case "starts_with":
		return fmt.Sprintf("starts with %q", value)
	case "ends_with":
		return fmt.Sprintf("ends with %q", value)
	}
	return fmt.Sprintf("is %q", value)
}

// matchDescription names the record that was found. The record's own Name (or
// the nearest equivalent on objects that have none) is far more useful in an
// execution log than an 18-character ID, so it leads with that where it can.
func matchDescription(record map[string]interface{}, id, matchField, matchValue string) string {
	for _, field := range []string{"Name", "Subject", "CaseNumber", "OrderNumber", "Title"} {
		if v, ok := record[field].(string); ok && strings.TrimSpace(v) != "" {
			return fmt.Sprintf("%q (%s)", v, id)
		}
	}
	if id != "" {
		return fmt.Sprintf("%s where %s matched %q", id, matchField, matchValue)
	}
	return fmt.Sprintf("matching %s %q", matchField, matchValue)
}

// defaultedFields resolves the projection when the operator has chosen none.
//
// BuildSelect's own fallback ends in "Id,Name,LastModifiedDate", and Name does
// not exist on Task, Event, Case, ContentDocument or any junction object — all
// hard INVALID_FIELD. This action is pointed at an arbitrary object by
// definition, so that guess is wrong exactly when it matters. Asking describe
// costs nothing extra: the same response is already cached for field typing.
func defaultedFields(instanceURL, token, object, chosen string) string {
	if strings.TrimSpace(chosen) != "" {
		return chosen
	}
	return salesforce.DefaultFieldsFor(instanceURL, token, object)
}
