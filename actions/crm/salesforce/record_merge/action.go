// Package crm_salesforce_record_merge merges duplicate records into one
// surviving master.
//
// De-duplication is the number one CRM hygiene chore — web-to-lead forms and
// front-desk staff generate duplicate accounts and contacts relentlessly — and
// like undelete it has no REST endpoint. It goes through the shared SOAP bridge
// in common.go on the same OAuth access token.
//
// What Salesforce does on a merge, and why it is not the same as deleting the
// loser: the master keeps its own field values (unless this action is given
// replacements), the merged records' related items — activities, cases, notes,
// attachments, campaign memberships — are RE-PARENTED onto the master, the
// losers go to the Recycle Bin, and Salesforce records the master's ID as the
// loser's MasterRecordId so anything holding the old ID can still find it.
//
// Salesforce's limits, enforced here rather than discovered at runtime: only
// Account, Contact, Lead and Case can be merged, all records must be the same
// object, and at most two records can be merged into the master per call.
package crm_salesforce_record_merge

import (
	"encoding/xml"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Merge Duplicate Records"
	Description  = "Merge duplicate accounts, contacts, leads or cases into one surviving record. Activities, notes and related items move across to the record you keep; the duplicates go to the Recycle Bin."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+link"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{
		Name:     "object",
		Type:     core.ConnectionTypeString,
		Label:    "Salesforce Object",
		Required: true,
		// A closed list, unlike every other action here: Salesforce only
		// supports merging these four, so offering the org's full object list
		// would be offering configurations that can never work.
		Options: []core.ConnectionOption{
			{Name: "Account", Value: "Account"},
			{Name: "Contact", Value: "Contact"},
			{Name: "Lead", Value: "Lead"},
			{Name: "Case", Value: "Case"},
		},
	},
	{Name: "master_record_id", Type: core.ConnectionTypeString, Label: "Record To Keep", Placeholder: "0015f00000AbCdEAAV — the record that survives the merge", Required: true},
	{Name: "merge_record_ids", Type: core.ConnectionTypeString, Label: "Records To Merge In", Placeholder: "0015f00000XyZ... — up to two, comma separated", Required: true},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Fields To Set On The Record You Keep", Placeholder: "{\"Phone\":\"0161 496 0000\"} — optional, applied as part of the merge"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Surviving Record ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Raw Response"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// mergeEnvelope models the Partner API's mergeResponse. Go's XML decoder
// matches on LOCAL names, so Salesforce's namespace prefixes need no modelling.
type mergeEnvelope struct {
	Results []struct {
		ID                string   `xml:"id"`
		Success           bool     `xml:"success"`
		MergedRecordIds   []string `xml:"mergedRecordIds"`
		UpdatedRelatedIds []string `xml:"updatedRelatedIds"`
		Errors            []struct {
			StatusCode string   `xml:"statusCode"`
			Message    string   `xml:"message"`
			Fields     []string `xml:"fields"`
		} `xml:"errors"`
	} `xml:"Body>mergeResponse>result"`
}

// mergeableObjects is Salesforce's own list for the SOAP merge() call.
var mergeableObjects = map[string]string{
	"account": "Account",
	"contact": "Contact",
	"lead":    "Lead",
	"case":    "Case",
}

// maxMergeRecords is Salesforce's per-call ceiling: a master plus at most two
// records to merge into it. Merging more means several calls, each against the
// surviving master.
const maxMergeRecords = 2

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	rawObject, err := salesforce.RequiredString("object", inputs)
	if err != nil {
		return nil, err
	}
	object, ok := mergeableObjects[strings.ToLower(rawObject)]
	if !ok {
		return nil, fmt.Errorf("Salesforce can only merge Account, Contact, Lead or Case records — %q cannot be merged. For other objects, move what you need across and delete the duplicate", rawObject)
	}

	masterID, err := salesforce.RequiredString("master_record_id", inputs)
	if err != nil {
		return nil, fmt.Errorf("the record to keep is required — the merge needs a surviving master record")
	}
	if err := salesforce.ValidateRecordID(masterID); err != nil {
		return nil, err
	}

	mergeIDs := salesforce.SplitList(salesforce.OptionalString("merge_record_ids", inputs))
	if len(mergeIDs) == 0 {
		return nil, fmt.Errorf("at least one record to merge in is required — otherwise there is nothing to merge")
	}
	if len(mergeIDs) > maxMergeRecords {
		return nil, fmt.Errorf("Salesforce merges at most %d records into the master per call — %d were given. Run this action more than once, keeping the same record each time", maxMergeRecords, len(mergeIDs))
	}
	for i, id := range mergeIDs {
		if err := salesforce.ValidateRecordID(id); err != nil {
			return nil, fmt.Errorf("record to merge in %d: %w", i+1, err)
		}
		if sameRecord(id, masterID) {
			return nil, fmt.Errorf("%s is both the record to keep and a record to merge in — a record cannot be merged into itself", id)
		}
	}

	fields := map[string]interface{}{}
	if err := salesforce.MergeAdditionalFields(fields, inputs); err != nil {
		return nil, err
	}
	// Field names become XML element names in the SOAP body, so they are
	// whitelist-validated exactly like a SOQL identifier — and a dotted
	// relationship path, legal in SOQL, is rejected because merge writes to the
	// master record's own fields only.
	for _, name := range salesforce.SortedKeys(fields) {
		if _, err := salesforce.ValidateSOQLFieldName(name); err != nil {
			return nil, err
		}
		if strings.Contains(name, ".") {
			return nil, fmt.Errorf("%q cannot be set during a merge — only fields on the %s record itself can be changed, not fields on a related record", name, object)
		}
	}

	respXML, err := salesforce.SOAPCall(instanceURL, token, buildMergeBody(object, masterID, mergeIDs, fields))
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	var env mergeEnvelope
	if err := xml.Unmarshal(respXML, &env); err != nil {
		return salesforce.ErrorResult(fmt.Sprintf("could not read Salesforce's response to the merge: %v", err)), nil
	}
	if len(env.Results) == 0 {
		return salesforce.ErrorResult("Salesforce returned no result for the merge"), nil
	}

	result := env.Results[0]
	if !result.Success {
		// A refused merge is a provider decision (locked record, cross-object
		// IDs, insufficient access), not something the operator mis-typed.
		messages := make([]string, 0, len(result.Errors))
		for _, e := range result.Errors {
			msg := strings.TrimSpace(e.Message)
			if msg == "" {
				msg = e.StatusCode
			}
			if len(e.Fields) > 0 {
				msg += " — field(s): " + strings.Join(e.Fields, ", ")
			}
			messages = append(messages, msg)
		}
		detail := strings.Join(messages, "; ")
		if detail == "" {
			detail = "Salesforce declined the merge without giving a reason"
		}
		return salesforce.ErrorResult(fmt.Sprintf("could not merge into %s %s: %s", object, masterID, detail)), nil
	}

	survivingID := result.ID
	if survivingID == "" {
		survivingID = masterID
	}
	raw := map[string]interface{}{
		"id":                survivingID,
		"success":           true,
		"mergedRecordIds":   result.MergedRecordIds,
		"updatedRelatedIds": result.UpdatedRelatedIds,
	}
	summary := fmt.Sprintf("Merged %d duplicate %s record(s) into %s; %d related record(s) were moved across", len(result.MergedRecordIds), object, survivingID, len(result.UpdatedRelatedIds))
	return salesforce.RecordResult(survivingID, raw, summary), nil
}

// sameRecord reports whether two IDs name the same record.
//
// Comparison is CASE-SENSITIVE on the first 15 characters, which is the only
// correct way to do it: the 15-character form of a Salesforce ID is
// case-sensitive (an unqualified case-insensitive compare would reject two
// genuinely different records), while the 18-character form just appends a
// case-safe checksum to the same 15 characters — so trimming to 15 is what
// makes the two forms of one record compare equal.
func sameRecord(a, b string) bool {
	trim := func(id string) string {
		id = strings.TrimSpace(id)
		if len(id) > 15 {
			return id[:15]
		}
		return id
	}
	return trim(a) == trim(b)
}

// buildMergeBody assembles the Partner API merge request.
//
// The master is an sObject, and the partner schema fixes the order of its
// children: type, then any fields being cleared, then Id, then the fields being
// set. Field elements have to be in the sObject namespace (the schema's <any>
// is restricted to it), which is what the sob: prefix declared on masterRecord
// is for. A JSON null in the fields object becomes a fieldsToNull entry —
// Salesforce's only way to clear a field, since an omitted field means "leave
// alone" and an empty string is a value in its own right.
func buildMergeBody(object, masterID string, mergeIDs []string, fields map[string]interface{}) string {
	var b strings.Builder
	b.WriteString("<urn:merge><urn:request>")
	b.WriteString(`<urn:masterRecord xmlns:sob="urn:sobject.partner.soap.sforce.com" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="sob:sObject">`)
	b.WriteString("<sob:type>" + salesforce.XMLEscape(object) + "</sob:type>")

	names := salesforce.SortedKeys(fields)
	for _, name := range names {
		if fields[name] == nil {
			b.WriteString("<sob:fieldsToNull>" + salesforce.XMLEscape(name) + "</sob:fieldsToNull>")
		}
	}
	b.WriteString("<sob:Id>" + salesforce.XMLEscape(masterID) + "</sob:Id>")
	for _, name := range names {
		v := fields[name]
		if v == nil {
			continue
		}
		escaped := salesforce.XMLEscape(name)
		b.WriteString("<sob:" + escaped + ">" + salesforce.XMLEscape(fmt.Sprintf("%v", v)) + "</sob:" + escaped + ">")
	}
	b.WriteString("</urn:masterRecord>")

	for _, id := range mergeIDs {
		b.WriteString("<urn:recordToMergeIds>" + salesforce.XMLEscape(id) + "</urn:recordToMergeIds>")
	}
	b.WriteString("</urn:request></urn:merge>")
	return b.String()
}
