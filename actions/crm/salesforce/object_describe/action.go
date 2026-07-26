// Package crm_salesforce_object_describe returns an object's full metadata:
// its fields, which are required, their picklist values, its record types and
// its related lists.
//
// One action covers every object rather than a describe action per object. It
// is what an operator reaches for to answer the three questions that block a
// flow: what is this field actually called, what values will Salesforce accept
// in it, and what is the related list named so Get Related Records can use it.
//
// Two things about describe that are easy to be caught out by. It is filtered
// by the CONNECTED user's permissions — a field the admin who built the flow
// can see may simply be absent when the flow runs as someone else, with no
// error. And the flat field describe lists EVERY picklist value on the object,
// including ones a given record type hides; supplying a record type ID switches
// this to the layout describe, which is the only view that reflects what the
// user would actually see on screen.
package crm_salesforce_object_describe

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Describe Object"
	Description  = "Look up everything about a Salesforce object — its fields, which are required, the values a dropdown field accepts, its record types and its related lists. Works for any standard or custom object."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+clipboard-list"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "object", Type: core.ConnectionTypeString, Label: "Salesforce Object", Placeholder: "Account, Contact, Opportunity — or a custom one like Invoice__c", Required: true},
	{Name: "picklist_field", Type: core.ConnectionTypeString, Label: "Dropdown Field", Placeholder: "Status, StageName — optional, pulls out just that field's accepted values"},
	{Name: "record_type_id", Type: core.ConnectionTypeString, Label: "Record Type ID", Placeholder: "012... — optional, returns the page layout for that record type instead"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Record ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Object Metadata"},
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
	object, err = salesforce.ValidateSOQLObjectName(object)
	if err != nil {
		return nil, err
	}

	// A record type turns this into a LAYOUT describe. Picklist values are
	// record-type dependent, and the ordinary describe cannot express that —
	// it returns the union of every record type's values, so a flow built from
	// it can offer a status the record type in question does not allow.
	if recordTypeID := salesforce.OptionalString("record_type_id", inputs); recordTypeID != "" {
		if err := salesforce.ValidateRecordID(recordTypeID); err != nil {
			return nil, fmt.Errorf("the record type ID is not valid: %w", err)
		}
		layout, err := describeLayout(instanceURL, token, object, recordTypeID)
		if err != nil {
			return salesforce.ErrorResult(err.Error()), nil
		}
		return salesforce.RecordResult("", layout, fmt.Sprintf("Described the %s page layout for record type %s", object, recordTypeID)), nil
	}

	describe, err := salesforce.DescribeObject(instanceURL, token, object)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf("Described %s: %d field(s), %d related list(s)", object, countList(describe, "fields"), countList(describe, "childRelationships"))

	// The Dropdown Field shortcut answers "what can I put in this field?"
	// without the operator scrolling a 400-field describe. It ADDS a key rather
	// than replacing the response, so the rest of the metadata stays reachable
	// and the output shape does not change with the input. Describe has no
	// top-level picklistValues of its own, so there is nothing to clobber.
	if field := salesforce.OptionalString("picklist_field", inputs); field != "" {
		// The two ways this can be wrong are different mistakes and need
		// different words. PicklistValues cannot tell them apart — it returns a
		// non-nil EMPTY slice for any field that exists, whatever its type, so
		// asking about Name or AnnualRevenue used to report success and
		// "accepts 0 value(s)" with picklistValues: []. That is not terse, it is
		// false: AnnualRevenue accepts any number. Resolve the field from the
		// describe payload directly and say which mistake was made.
		meta, found := findField(describe, field)
		switch {
		case !found:
			return nil, fmt.Errorf("%s has no field called %q — check the spelling against the field list in this action's output, or the connected Salesforce user may not be allowed to see it", object, field)
		case !isPicklistType(fieldType(meta)):
			return nil, fmt.Errorf("%s on %s is not a dropdown — it is a %s, so it has no fixed list of values. Leave Dropdown Field blank to see every field on the object", fieldLabel(meta, field), object, friendlyType(fieldType(meta)))
		}

		values := salesforce.PicklistValues(describe, field)
		items := make([]interface{}, 0, len(values))
		for _, v := range values {
			items = append(items, v)
		}
		describe["picklistValues"] = items
		summary = fmt.Sprintf("Described %s: %s accepts %d value(s)", object, field, len(items))
		if len(items) == 0 {
			// A real picklist with nothing in it. Worth saying out loud, because
			// "accepts 0 value(s)" reads like a bug rather than like an empty
			// list somebody has to populate in Setup.
			summary = fmt.Sprintf("Described %s: %s is a dropdown with no values set up in your org", object, field)
		}
	}

	return salesforce.RecordResult("", describe, summary), nil
}

// describeLayout fetches the page layout for one record type, which is the only
// view of an object whose picklists are filtered the way the Salesforce UI
// filters them.
func describeLayout(instanceURL, token, object, recordTypeID string) (map[string]interface{}, error) {
	path := "/sobjects/" + object + "/describe/layouts/" + url.PathEscape(recordTypeID)
	resp, err := salesforce.ExecuteAPI(instanceURL, token, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if err := salesforce.CheckResponse(resp); err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("could not read the %s page layout from Salesforce: %w", object, err)
	}
	return out, nil
}

// findField locates one field in a describe payload by API name, case-insensitively
// — the same match PicklistValues makes, so the two never disagree about which
// field is being talked about.
func findField(describe map[string]interface{}, name string) (map[string]interface{}, bool) {
	fields, _ := describe["fields"].([]interface{})
	for _, f := range fields {
		fm, ok := f.(map[string]interface{})
		if !ok {
			continue
		}
		if fn, _ := fm["name"].(string); strings.EqualFold(fn, name) {
			return fm, true
		}
	}
	return nil, false
}

func fieldType(meta map[string]interface{}) string {
	t, _ := meta["type"].(string)
	return t
}

// fieldLabel prefers the on-screen label, which is what the operator was
// looking at when they picked the field.
func fieldLabel(meta map[string]interface{}, fallback string) string {
	if l, _ := meta["label"].(string); l != "" {
		return l
	}
	return fallback
}

// picklistTypes are the Salesforce field types that carry a fixed list of
// values. combobox is included: it offers the list and also accepts free text,
// so "what values does this take?" is still a fair question about it.
var picklistTypes = map[string]bool{
	"picklist":      true,
	"multipicklist": true,
	"combobox":      true,
}

func isPicklistType(t string) bool { return picklistTypes[strings.ToLower(t)] }

// friendlyType turns Salesforce's own type names into something a front-of-house
// operator can act on. Unrecognised types fall through unchanged rather than
// being guessed at — "it is a %s field" with the raw name still beats silence.
func friendlyType(t string) string {
	switch strings.ToLower(t) {
	case "string":
		return "text field"
	case "textarea":
		return "long text field"
	case "reference":
		return "link to another record"
	case "id":
		return "record ID"
	case "double", "int", "percent":
		return "number field"
	case "currency":
		return "money field"
	case "boolean":
		return "tick box"
	case "date":
		return "date field"
	case "datetime":
		return "date and time field"
	case "email":
		return "email field"
	case "phone":
		return "phone field"
	case "url":
		return "web address field"
	case "":
		return "field of an unknown type"
	}
	return t + " field"
}

// countList reports how many entries a named array in the describe response
// holds, for the summary line.
func countList(describe map[string]interface{}, key string) int {
	list, _ := describe[key].([]interface{})
	return len(list)
}
