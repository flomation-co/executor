// Package crm_salesforce_record_get_related lists the child records hanging off
// a parent record — an account's contacts, an opportunity's line items, a
// contact's cases.
//
// This is the biggest single usability unlock in the node. Salesforce exposes
// related lists at /sobjects/{object}/{id}/{RelationshipName}, which returns the
// same envelope a SOQL query does; without it a non-technical operator has to
// hand-write "SELECT Id, Name FROM Contact WHERE AccountId = '001...'" just to
// answer "who works at this company?".
//
// The relationship NAME is not the child object's name: Account's contacts live
// under "Contacts", its deals under "Opportunities", and a custom child under
// "My_Children__r". The names come from the parent object's describe response
// (childRelationships[].relationshipName), which is why the Describe Object
// action sits next to this one in the palette.
package crm_salesforce_record_get_related

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
	Name         = "Salesforce: Get Related Records"
	Description  = "List the records linked to a parent record — the contacts at an account, the deals on a company, the cases for a customer. Pick the parent and the relationship; no query writing needed."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+diagram-project"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "object", Type: core.ConnectionTypeString, Label: "Parent Object", Placeholder: "Account, Contact, Opportunity — or a custom one like Invoice__c", Required: true},
	{Name: "record_id", Type: core.ConnectionTypeString, Label: "Parent Record ID", Placeholder: "0015f00000AbCdEAAV", Required: true},
	{Name: "relationship_name", Type: core.ConnectionTypeString, Label: "Related List", Placeholder: "Contacts, Opportunities, Cases — or a custom one like Invoices__r", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Id, Name, Email — comma separated (blank returns Salesforce's default set)"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All (fetch every page)"},
	{Name: "page_url", Type: core.ConnectionTypeString, Label: "Next Page URL (resume)", Placeholder: "The Next Page URL from a previous run — leave blank to start from the beginning"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Related Records"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total_size", Type: core.ConnectionTypeInteger, Label: "Total Available"},
	{Name: "next_url", Type: core.ConnectionTypeString, Label: "Next Page URL"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Raw Response"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// relatedResponse is the envelope a related-list GET returns. It is the same
// shape as a SOQL query result, cursor included — Salesforce pages related
// lists exactly like a query.
type relatedResponse struct {
	TotalSize      int                      `json:"totalSize"`
	Done           bool                     `json:"done"`
	NextRecordsURL string                   `json:"nextRecordsUrl"`
	Records        []map[string]interface{} `json:"records"`
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
	recordID, err := salesforce.RequiredString("record_id", inputs)
	if err != nil {
		return nil, err
	}
	if err := salesforce.ValidateRecordID(recordID); err != nil {
		return nil, err
	}
	relationship, err := salesforce.RequiredString("relationship_name", inputs)
	if err != nil {
		return nil, err
	}
	// The relationship name goes straight into the request path, so it is
	// whitelist-validated the same way a SOQL identifier is — it is operator
	// input and it is never quoted. Validate into a new variable so the
	// original spelling survives for the error message.
	validRelationship, err := salesforce.ValidateSOQLFieldName(relationship)
	if err != nil {
		return nil, fmt.Errorf("%q is not a valid related list name — use the name from the parent object's describe, e.g. Contacts or Invoices__r", relationship)
	}
	relationship = validRelationship

	path := "/sobjects/" + object + "/" + url.PathEscape(recordID) + "/" + relationship
	fields, err := validatedFields(salesforce.OptionalString("fields", inputs))
	if err != nil {
		return nil, err
	}
	if fields != "" {
		path += "?fields=" + url.QueryEscape(fields)
	}

	returnAll := salesforce.OptionalBool("return_all", inputs)
	records := []map[string]interface{}{}
	nextURL := ""
	totalSize := 0
	pages := 0
	absolute := false

	// A resume cursor from an earlier run replaces the first request entirely:
	// Salesforce's nextRecordsUrl already carries the object, the record, the
	// relationship, the field list and the version, so re-deriving any of them
	// here would only be a chance to disagree with it.
	if resume := salesforce.OptionalString("page_url", inputs); resume != "" {
		if nextURL, err = validateCursor(resume); err != nil {
			return nil, err
		}
		absolute = true
	}

	for {
		var resp *salesforce.APIResponse
		if absolute {
			// nextRecordsUrl comes back rooted at the instance with its own
			// /services/data/vNN prefix already attached.
			resp, err = salesforce.ExecuteAbsolute(instanceURL, token, http.MethodGet, nextURL, nil)
		} else {
			resp, err = salesforce.ExecuteAPI(instanceURL, token, http.MethodGet, path, nil)
		}
		if err != nil {
			return salesforce.ErrorResult(err.Error()), nil
		}
		if err := salesforce.CheckResponse(resp); err != nil {
			return salesforce.ErrorResult(err.Error()), nil
		}
		var rr relatedResponse
		if err := json.Unmarshal(resp.Body, &rr); err != nil {
			return salesforce.ErrorResult(fmt.Sprintf("could not read the %s related list from Salesforce: %v", relationship, err)), nil
		}
		pages++
		records = append(records, rr.Records...)
		totalSize = rr.TotalSize
		nextURL = rr.NextRecordsURL

		if rr.Done || nextURL == "" || !returnAll || pages >= salesforce.MaxAllPages {
			break
		}
		absolute = true
	}

	summary := fmt.Sprintf("Found %d %s record(s) on %s %s", len(records), relationship, object, recordID)
	if returnAll && nextURL != "" && pages >= salesforce.MaxAllPages {
		summary = fmt.Sprintf("Fetched %d %s record(s) across %d page(s); stopped at the %d-page safety cap — more remain", len(records), relationship, pages, salesforce.MaxAllPages)
	} else if returnAll {
		summary = fmt.Sprintf("Fetched all %d %s record(s) on %s %s across %d page(s)", len(records), relationship, object, recordID, pages)
	}
	return salesforce.ListResult(records, nextURL, totalSize, summary), nil
}

// validateCursor checks a resume URL before it is appended to the org's origin.
//
// The leading slash is the load-bearing part: the cursor is joined onto
// "https://mycompany.my.salesforce.com", so a value beginning "@elsewhere" would
// turn the org's host into userinfo and send the access token somewhere else.
func validateCursor(raw string) (string, error) {
	cursor := strings.TrimSpace(raw)
	if strings.Contains(cursor, "://") || !strings.HasPrefix(cursor, "/") || strings.HasPrefix(cursor, "//") {
		return "", fmt.Errorf("the next page URL is not one Salesforce produced — pass the Next Page URL output of an earlier run unchanged, e.g. /services/data/v62.0/query/01g5f000000gG4bAAC-2000")
	}
	return cursor, nil
}

// validatedFields turns the comma-separated Fields input into a validated
// field list. Identifiers cannot be quoted in a Salesforce request, so
// whitelisting them is the only defence available.
func validatedFields(raw string) (string, error) {
	names := salesforce.SplitList(raw)
	if len(names) == 0 {
		return "", nil
	}
	validated := make([]string, 0, len(names))
	for _, n := range names {
		f, err := salesforce.ValidateSOQLFieldName(n)
		if err != nil {
			return "", err
		}
		validated = append(validated, f)
	}
	return strings.Join(validated, ","), nil
}
