package crm_salesforce_opportunity_describe

import (
	"encoding/json"
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Opportunity Metadata"
	Description  = "Return how the Opportunity object is set up in your Salesforce org, along with the deals the connected user looked at most recently."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+clipboard-list"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank — taken from your connection", FromCredentialMeta: "instance_url"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Object Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Metadata And Recent Deals"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	// The sObject Basic Information resource — /sobjects/Opportunity with no ID
	// and no /describe suffix. It answers {objectDescribe, recentItems}: the
	// object's headline metadata (can this user create deals? what is the object
	// called in this org's language?) plus the deals the connected user opened
	// most recently. n8n calls this "Get Summary", which is misleading — it
	// summarises nothing about the pipeline.
	resp, err := salesforce.ExecuteAPI(instanceURL, token, http.MethodGet, "/sobjects/Opportunity", nil)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	if err := salesforce.CheckResponse(resp); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	var record map[string]interface{}
	if err := json.Unmarshal(resp.Body, &record); err != nil {
		return salesforce.ErrorResult(fmt.Sprintf("failed to parse the Salesforce metadata response: %v", err)), nil
	}

	label := "Opportunity"
	if describe, ok := record["objectDescribe"].(map[string]interface{}); ok {
		if l, ok := describe["label"].(string); ok && l != "" {
			// Orgs rename the object (to "Deal", commonly), and the operator
			// recognises their own label, not Salesforce's.
			label = l
		}
	}
	recent, _ := record["recentItems"].([]interface{})

	summary := fmt.Sprintf("Retrieved %s object metadata and %d recently-viewed record(s)", label, len(recent))
	return salesforce.RecordResult("Opportunity", record, summary), nil
}
