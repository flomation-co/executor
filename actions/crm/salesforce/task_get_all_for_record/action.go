package crm_salesforce_task_get_all_for_record

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
	Name         = "Salesforce: Get Tasks On a Record"
	Description  = "List the tasks logged against one contact, lead, account, opportunity or case — open and completed alike. This is what a follow-up chaser needs: point it at the customer and get back everything outstanding."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+clipboard-list"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

// tasksRelationship is the name Salesforce gives the Task related list on every
// object that can carry activities. Traversing the relationship rather than
// querying Task directly is what keeps this correct for both halves of
// Salesforce's split: tasks hang off a person through WhoId and off everything
// else through WhatId, and the relationship endpoint knows which applies.
const tasksRelationship = "/Tasks"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "object", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "Contact, Lead, Account, Opportunity or Case", Required: true},
	{Name: "record_id", Type: core.ConnectionTypeString, Label: "Record ID", Placeholder: "0035f00000AbCdEEAV — the contact, account or deal the tasks are on", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields To Return", Placeholder: "Id, Subject, Status, ActivityDate — leave blank for the usual ones"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 (max 2000) — ignored when Return All is on"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All (fetch every task on the record)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Tasks"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total_size", Type: core.ConnectionTypeInteger, Label: "Records Returned"},
	{Name: "next_url", Type: core.ConnectionTypeString, Label: "Next Page URL"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Raw Response"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// relatedTasksPage is one page of Salesforce's related-list response. It is the
// same envelope a SOQL query returns, cursor and all, which is why paging works
// identically here.
type relatedTasksPage struct {
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

	object := salesforce.OptionalString("object", inputs)
	if object == "" {
		return nil, fmt.Errorf("object is required — the kind of record the tasks are logged against, e.g. Contact, Lead, Account or Opportunity")
	}
	obj, err := salesforce.ValidateSOQLObjectName(object)
	if err != nil {
		return nil, err
	}
	recordID := salesforce.OptionalString("record_id", inputs)
	if err := salesforce.ValidateRecordID(recordID); err != nil {
		return nil, err
	}

	// The related-list resource has no default column set worth relying on, so
	// the field list is always sent — falling back to the same default a
	// get-many uses. Every name is whitelist-validated first: it lands in a URL
	// Salesforce parses as field identifiers, so it is the same boundary a
	// hand-written query would have to respect.
	fields := salesforce.OptionalString("fields", inputs)
	if strings.TrimSpace(fields) == "" {
		fields = salesforce.DefaultFields("Task")
	}
	validated := make([]string, 0, 8)
	for _, f := range salesforce.SplitList(fields) {
		v, err := salesforce.ValidateSOQLFieldName(f)
		if err != nil {
			return nil, err
		}
		validated = append(validated, v)
	}
	if len(validated) == 0 {
		validated = append(validated, "Id")
	}

	returnAll := salesforce.OptionalBool("return_all", inputs)
	limit, limitSet := salesforce.OptionalInt("limit", inputs)
	maxRecords := salesforce.ClampLimit(limit, limitSet)

	path := "/sobjects/" + obj + "/" + url.PathEscape(recordID) + tasksRelationship + "?fields=" + url.QueryEscape(strings.Join(validated, ","))

	records := []map[string]interface{}{}
	nextURL := ""
	totalSize := 0
	pages := 0
	// Page one is relative to the API version root; every page after it uses the
	// absolute path Salesforce hands back, which already carries its own version
	// prefix and must not be prefixed again.
	absolute := false
	for {
		var resp *salesforce.APIResponse
		if absolute {
			resp, err = salesforce.ExecuteAbsolute(instanceURL, token, http.MethodGet, path, nil)
		} else {
			resp, err = salesforce.ExecuteAPI(instanceURL, token, http.MethodGet, path, nil)
		}
		if err != nil {
			return salesforce.ErrorResult(err.Error()), nil
		}
		if err := salesforce.CheckResponse(resp); err != nil {
			return salesforce.ErrorResult(err.Error()), nil
		}
		var page relatedTasksPage
		if err := json.Unmarshal(resp.Body, &page); err != nil {
			return salesforce.ErrorResult(fmt.Sprintf("failed to parse Salesforce response: %s", err.Error())), nil
		}
		pages++
		records = append(records, page.Records...)
		totalSize = page.TotalSize
		nextURL = page.NextRecordsURL

		if !returnAll || page.Done || nextURL == "" || pages >= salesforce.MaxAllPages {
			break
		}
		path = nextURL
		absolute = true
	}

	// Salesforce sizes related-list pages itself — there is no limit parameter on
	// this resource — so the Limit is applied here. next_url is left as Salesforce
	// gave it, so a later run can still pick up where this one stopped.
	if !returnAll && len(records) > maxRecords {
		records = records[:maxRecords]
	}

	out := salesforce.ListResult(records, nextURL, totalSize, "")
	switch {
	case returnAll && nextURL != "" && pages >= salesforce.MaxAllPages:
		out["tool_result"] = fmt.Sprintf("Fetched %d task(s) on %s %s across %d page(s); stopped at the %d-page safety cap", len(records), obj, recordID, pages, salesforce.MaxAllPages)
	case returnAll:
		out["tool_result"] = fmt.Sprintf("Fetched all %d task(s) on %s %s", len(records), obj, recordID)
	default:
		out["tool_result"] = fmt.Sprintf("Found %d task(s) on %s %s", len(records), obj, recordID)
	}
	return out, nil
}
