// Package crm_salesforce_record_undelete restores a record from the Salesforce
// Recycle Bin.
//
// This is a genuine emergency verb — "someone deleted the wrong account" — and
// there is no REST endpoint for it at all. Salesforce only exposes undelete on
// the Partner SOAP API, so this action goes through the shared SOAP bridge in
// common.go, reusing the very same OAuth access token as the SOAP <sessionId>.
// No second credential and no extra setup for the operator.
//
// The window is 15 days: after that Salesforce empties the Recycle Bin and the
// record is gone for good. Get Deleted Records finds the IDs to pass in here.
package crm_salesforce_record_undelete

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
	Name         = "Salesforce: Restore Deleted Record"
	Description  = "Bring a record back from the Salesforce Recycle Bin after it was deleted by mistake. Works for 15 days after the deletion, on any object."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+rotate-left"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "record_id", Type: core.ConnectionTypeString, Label: "Record ID", Placeholder: "0015f00000AbCdEAAV — the ID of the deleted record", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Record ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Raw Response"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// undeleteEnvelope models the Partner API's undeleteResponse. Go's XML decoder
// matches on LOCAL names, so the soapenv/urn namespace prefixes Salesforce
// sends do not have to be modelled here.
type undeleteEnvelope struct {
	Results []struct {
		ID      string `xml:"id"`
		Success bool   `xml:"success"`
		Errors  []struct {
			StatusCode string   `xml:"statusCode"`
			Message    string   `xml:"message"`
			Fields     []string `xml:"fields"`
		} `xml:"errors"`
	} `xml:"Body>undeleteResponse>result"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	recordID, err := salesforce.RequiredString("record_id", inputs)
	if err != nil {
		return nil, fmt.Errorf("record_id is required — the Salesforce ID of the record to bring back. Get Deleted Records lists the IDs of recently deleted records")
	}
	if err := salesforce.ValidateRecordID(recordID); err != nil {
		return nil, err
	}

	// The SOAP body is assembled as a string, so the ID is escaped on the way
	// in — the same boundary the SOQL builder guards for queries. The ID is
	// already known to be alphanumeric, but the escape costs nothing and the
	// habit is what keeps the next body that takes free text safe.
	body := "<urn:undelete><urn:ids>" + salesforce.XMLEscape(recordID) + "</urn:ids></urn:undelete>"
	respXML, err := salesforce.SOAPCall(instanceURL, token, body)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	var env undeleteEnvelope
	if err := xml.Unmarshal(respXML, &env); err != nil {
		return salesforce.ErrorResult(fmt.Sprintf("could not read Salesforce's response to the restore: %v", err)), nil
	}
	if len(env.Results) == 0 {
		return salesforce.ErrorResult("Salesforce returned no result for the restore — the record may never have existed"), nil
	}

	result := env.Results[0]
	if !result.Success {
		// A failed undelete is a provider decision, not a configuration
		// mistake: the commonest cause is that the record has already aged out
		// of the Recycle Bin, which the operator cannot fix by editing inputs.
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
			detail = "Salesforce declined the restore without giving a reason"
		}
		return salesforce.ErrorResult(fmt.Sprintf("could not restore %s: %s. Records stay in the Recycle Bin for 15 days; after that they cannot be brought back", recordID, detail)), nil
	}

	restoredID := result.ID
	if restoredID == "" {
		restoredID = recordID
	}
	raw := map[string]interface{}{"id": restoredID, "success": true}
	return salesforce.RecordResult(restoredID, raw, fmt.Sprintf("Restored record %s from the Recycle Bin", restoredID)), nil
}
