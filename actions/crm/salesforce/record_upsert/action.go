// Package crm_salesforce_record_upsert creates-or-updates a record on ANY
// standard or custom Salesforce object, matched on an external ID field.
//
// This is the idempotency primitive the rest of the node leans on. A webhook
// that fires twice, a scheduled sync that re-runs, or a spreadsheet imported a
// second time all land on the SAME Salesforce record instead of quietly
// creating a duplicate — which is the single most common way a no-code CRM
// integration corrupts a customer's data. n8n offers upsert on five objects
// only; here it works on every object in the org.
package crm_salesforce_record_upsert

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
	Name         = "Salesforce: Create or Update Record"
	Description  = "Create a record, or update the existing one that matches — on any Salesforce object. Salesforce matches on a field you choose (an order number, a customer reference, an email), so re-running the flow never creates a duplicate."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+rotate"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "object", Type: core.ConnectionTypeString, Label: "Salesforce Object", Placeholder: "Account, Contact, Opportunity — or a custom one like Invoice__c", Required: true},
	{Name: "external_id_field", Type: core.ConnectionTypeString, Label: "Match On Field", Placeholder: "The field Salesforce matches on, e.g. Customer_Ref__c", Required: true},
	{Name: "external_id_value", Type: core.ConnectionTypeString, Label: "Match On Value", Placeholder: "The value to look for, e.g. CUST-1042", Required: true},
	{Name: "fields", Type: core.ConnectionTypeObject, Label: "Fields", Placeholder: "{\"Name\":\"Acme Ltd\",\"Phone\":\"0161 496 0000\"}"},
	{Name: "record_type_id", Type: core.ConnectionTypeString, Label: "Record Type ID", Placeholder: "012... — leave blank to use the default record type"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Extra Fields", Placeholder: "{\"Custom_Field__c\":\"value\"} — merged on top of Fields"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Record ID"},
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

	object, err := salesforce.RequiredString("object", inputs)
	if err != nil {
		return nil, err
	}
	// Validate the identifiers here rather than letting UpsertRecord do it. A
	// misspelled object or field name is a configuration mistake and must be a
	// hard failure — routing it to the error port would make it look as though
	// Salesforce rejected a well-formed call.
	object, err = salesforce.ValidateSOQLObjectName(object)
	if err != nil {
		return nil, err
	}
	extField, err := salesforce.RequiredString("external_id_field", inputs)
	if err != nil {
		return nil, fmt.Errorf("the field to match on is required — pick the external ID field Salesforce should look the record up by")
	}
	extField, err = salesforce.ValidateSOQLFieldName(extField)
	if err != nil {
		return nil, err
	}
	extValue, err := salesforce.RequiredString("external_id_value", inputs)
	if err != nil {
		return nil, fmt.Errorf("the value to match on is required — it is what Salesforce searches %s for", extField)
	}

	body := map[string]interface{}{}
	if err := salesforce.MergeJSONObject(body, inputs, "fields"); err != nil {
		return nil, err
	}
	// An explicit record type wins over one buried in Fields, and Extra Fields
	// win over both — last writer takes the field, which is the order an
	// operator reading the panel top-to-bottom expects.
	salesforce.SetIfPresent(body, inputs, "RecordTypeId", "record_type_id")
	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("set at least one field — an upsert with no fields would create a record containing nothing but %s", extField)
	}

	id, created, raw, err := salesforce.UpsertRecord(instanceURL, token, object, extField, extValue, body)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// An upsert that MATCHED an existing record answers 204 No Content, so the
	// response carries no ID at all — unlike a create, where the caller can be
	// handed back what it just made. Resolve it with one GET on the same
	// external-ID URL: without an ID nothing downstream can chain off the
	// upsert, which is most of the reason to use one.
	if id == "" {
		if resolved, lookupErr := lookupByExternalID(instanceURL, token, object, extField, extValue); lookupErr == nil {
			id = salesforce.StringifyID(resolved["Id"])
			if len(raw) == 0 {
				raw = resolved
			}
		}
	}

	verb := "Updated"
	if created {
		verb = "Created"
	}
	summary := fmt.Sprintf("%s %s record %s (matched on %s = %s)", verb, object, id, extField, extValue)
	if id == "" {
		summary = fmt.Sprintf("%s the %s record matching %s = %s (Salesforce did not return its ID)", verb, object, extField, extValue)
	}
	return salesforce.RecordResult(id, raw, summary), nil
}

// escapeExternalID escapes the match value for a URL path segment.
//
// It MUST escape the value exactly the way the upsert itself did, or the
// recovery lookup addresses a different URL from the write it is recovering.
// url.PathEscape alone is not that: it leaves "+" as a literal, because "+"
// only means "space" in a query string and is legal in a path. The commonest
// external ID by far is an email address, plus-addressing is common in them,
// and anything along the way that treats the segment as form-encoded turns
// "a+b@x.com" into "a b@x.com" — so the shared UpsertRecord helper encodes it
// explicitly and this has to match. Getting this wrong is worse than returning
// no ID: the lookup can resolve a genuinely different record and hand its ID
// back as though it were the record just written.
func escapeExternalID(v string) string {
	return strings.ReplaceAll(url.PathEscape(v), "+", "%2B")
}

// lookupByExternalID fetches a record through its external-ID URL. Used only to
// recover the record ID after a 204 upsert, so a failure here is not fatal — the
// write already succeeded and the caller ignores the error.
func lookupByExternalID(instanceURL, token, object, extField, extValue string) (map[string]interface{}, error) {
	path := "/sobjects/" + object + "/" + extField + "/" + escapeExternalID(extValue)
	resp, err := salesforce.ExecuteAPI(instanceURL, token, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if err := salesforce.CheckResponse(resp); err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, err
	}
	return out, nil
}
