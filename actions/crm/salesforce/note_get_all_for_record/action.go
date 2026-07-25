// Package crm_salesforce_note_get_all_for_record reads the Enhanced Notes
// logged against a record — call summaries, handover notes, "what happened last
// time this customer rang".
//
// n8n can write a note (to the wrong object) and can never read one back, so
// the whole "brief the next person" half of the workflow is missing.
//
// Reading notes takes two queries, and that is Salesforce's fault rather than a
// design choice. ContentDocumentLink says WHICH notes are on the record but
// carries none of their content; ContentNote holds the title and text but
// cannot be filtered by the record it is attached to. So: list the links, then
// look the notes up by ID and stitch the two together.
package crm_salesforce_note_get_all_for_record

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Notes On A Record"
	Description  = "Read the notes logged against a Salesforce record — call summaries and handover notes — with a preview of each, or the full text on request."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+clipboard-list"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

const (
	// noteFileType is the ContentDocument file type Enhanced Notes are stored
	// as. Notes and files share one store, so this is what separates them.
	noteFileType = "SNOTE"

	// linkFields is the first query's SELECT list — just enough to identify the
	// notes and record how they are shared.
	linkFields = "Id,ContentDocumentId,LinkedEntityId,ShareType,Visibility"

	// noteFields is the second query's SELECT list. TextPreview is the first
	// 255 characters of the note as plain text, which is what makes a list of
	// notes readable without fetching every body.
	noteFields = "Id,Title,TextPreview,OwnerId,CreatedById,CreatedDate,LastModifiedDate"

	// maxBodyFetches bounds the full-text option: each note body is its own
	// request, so an unbounded loop over a busy account would burn the org's
	// daily API allowance on one step.
	maxBodyFetches = 25

	// maxIDsPerQuery chunks the second query's IN list. A SOQL query travels in
	// the request URI, and Salesforce rejects an over-long one outright — with
	// Limit set to its 2000 ceiling a single IN list of 18-character IDs is tens
	// of kilobytes once escaped, so the notes lookup would 414 on exactly the
	// busy record it was needed for. 200 matches the chunk size the Collections
	// helpers already use.
	maxIDsPerQuery = 200
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},

	// object never reaches Salesforce — the link is by record ID alone. It is
	// here so the editor can narrow the record picker to one object type.
	{Name: "object", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "Contact, Account, Case… — only used to help you pick the record"},
	{Name: "record_id", Type: core.ConnectionTypeString, Label: "Record", Placeholder: "The record whose notes you want, e.g. 0035f00000XyzAAB", Required: true},

	{Name: "include_body", Type: core.ConnectionTypeBoolean, Label: "Fetch The Full Note Text (slower)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 (max 2000)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Notes"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total_size", Type: core.ConnectionTypeInteger, Label: "Total Matching"},
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

	recordID, err := salesforce.RequiredString("record_id", inputs)
	if err != nil {
		return nil, fmt.Errorf("record_id is required — the record whose notes you want to read")
	}
	if err := salesforce.ValidateRecordID(recordID); err != nil {
		return nil, err
	}

	limit, limitSet := salesforce.OptionalInt("limit", inputs)
	// ContentDocumentLink accepts neither ORDER BY nor a paged Return All, so
	// this is one bounded page, sorted later by the notes query.
	linkSOQL, err := salesforce.BuildQuery("ContentDocumentLink", linkFields, []salesforce.Condition{
		{Field: "LinkedEntityId", Operator: "=", Value: recordID},
		{Field: "ContentDocument.FileType", Operator: "=", Value: noteFileType},
	}, false, "", salesforce.ClampLimit(limit, limitSet), true)
	if err != nil {
		return nil, err
	}

	links, nextURL, totalSize, _, err := salesforce.Query(instanceURL, token, linkSOQL, false, false)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	if len(links) == 0 {
		return salesforce.ListResult(nil, "", 0, fmt.Sprintf("No notes are logged against %s", recordID)), nil
	}

	notes, err := fetchNotes(instanceURL, token, links)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	if salesforce.OptionalBool("include_body", inputs) {
		attachBodies(instanceURL, token, notes)
	}

	return salesforce.ListResult(notes, nextURL, totalSize, fmt.Sprintf("Found %d note(s) on %s", len(notes), recordID)), nil
}

// fetchNotes looks the notes up by ID and merges each one's sharing settings
// back in from its link, so a single result row answers both "what does the
// note say" and "who can see it".
//
// Newest first: a note list is read to find out what happened most recently,
// and Salesforce's own default order is not that.
func fetchNotes(instanceURL, token string, links []map[string]interface{}) ([]map[string]interface{}, error) {
	ids := make([]string, 0, len(links))
	linkByNote := make(map[string]map[string]interface{}, len(links))
	for _, link := range links {
		id := salesforce.StringifyID(link["ContentDocumentId"])
		if id == "" {
			continue
		}
		ids = append(ids, id)
		linkByNote[id] = link
	}
	if len(ids) == 0 {
		return []map[string]interface{}{}, nil
	}

	// The IN list is chunked because the query rides in the request URI, and it
	// goes through the shared builder, which escapes and quotes each value —
	// these IDs came back from Salesforce, but the boundary is the boundary and
	// it costs nothing to keep it in one place.
	notes := make([]map[string]interface{}, 0, len(ids))
	chunks := 0
	for start := 0; start < len(ids); start += maxIDsPerQuery {
		end := start + maxIDsPerQuery
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[start:end]
		soql, err := salesforce.BuildQuery("ContentNote", noteFields, []salesforce.Condition{
			{Field: "Id", Operator: "IN", Value: strings.Join(chunk, ",")},
		}, false, "CreatedDate DESC", len(chunk), true)
		if err != nil {
			return nil, err
		}
		page, _, _, _, err := salesforce.Query(instanceURL, token, soql, false, false)
		if err != nil {
			return nil, err
		}
		notes = append(notes, page...)
		chunks++
	}
	// Each chunk is sorted by Salesforce, the concatenation of several is not.
	// Re-sorting here keeps the newest-first promise across chunk boundaries;
	// CreatedDate is a fixed-width UTC timestamp, so comparing it as text is
	// the same as comparing it as a time.
	if chunks > 1 {
		sort.SliceStable(notes, func(i, j int) bool {
			a, _ := notes[i]["CreatedDate"].(string)
			b, _ := notes[j]["CreatedDate"].(string)
			return a > b
		})
	}

	for _, note := range notes {
		link, ok := linkByNote[salesforce.StringifyID(note["Id"])]
		if !ok {
			continue
		}
		note["ShareType"] = link["ShareType"]
		note["Visibility"] = link["Visibility"]
		note["LinkedEntityId"] = link["LinkedEntityId"]
		note["ContentDocumentLinkId"] = link["Id"]
	}
	return notes, nil
}

// attachBodies fills in each note's full HTML text.
//
// The body is a blob field, so it is not queryable and needs one request per
// note — which is why it is opt-in and capped. Failures are recorded per note
// rather than failing the list: a single unreadable note should not cost the
// operator the other twenty-four.
func attachBodies(instanceURL, token string, notes []map[string]interface{}) {
	for i, note := range notes {
		if i >= maxBodyFetches {
			note["Body"] = ""
			note["BodyError"] = fmt.Sprintf("full text is only fetched for the first %d notes — narrow the list or read this one on its own", maxBodyFetches)
			continue
		}
		id := salesforce.StringifyID(note["Id"])
		if id == "" {
			continue
		}
		resp, err := salesforce.ExecuteAPI(instanceURL, token, http.MethodGet, "/sobjects/ContentNote/"+id+"/Content", nil)
		if err != nil {
			note["BodyError"] = err.Error()
			continue
		}
		if err := salesforce.CheckResponse(resp); err != nil {
			note["BodyError"] = err.Error()
			continue
		}
		note["Body"] = string(resp.Body)
	}
}
