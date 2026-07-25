package crm_salesforce_account_describe

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
	Name         = "Salesforce: Get Account Setup Info"
	Description  = "Find out how the Account object is set up in your Salesforce org — the accounts you viewed most recently, and (in Full detail) every field, picklist choice and record type available to build flows against."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+layer-group"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{
		Name:        "detail_level",
		Type:        core.ConnectionTypeString,
		Label:       "Detail",
		Placeholder: "Overview",
		Options: []core.ConnectionOption{
			{Name: "Overview (summary plus recently viewed accounts)", Value: "overview"},
			{Name: "Full (every field, picklist value and record type)", Value: "full"},
		},
	},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Object Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Account Setup"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	// n8n calls this operation "Get Summary", which is misleading: it summarises
	// the Account OBJECT, not an account. Two levels are offered instead —
	// the light basic-information call n8n makes, and the full describe, which
	// is what an operator actually wants when they are hunting for the API name
	// of a custom field or the exact spelling of a picklist value.
	full := salesforce.OptionalString("detail_level", inputs) == "full"

	// Both responses are filtered by the CONNECTED user's permissions: a field
	// that user cannot see is simply absent, so a flow built by an admin can
	// describe differently when it runs as someone else.
	if full {
		describe, err := salesforce.DescribeObject(instanceURL, token, "Account")
		if err != nil {
			return salesforce.ErrorResult(err.Error()), nil
		}
		fields, _ := describe["fields"].([]interface{})
		recordTypes, _ := describe["recordTypeInfos"].([]interface{})
		summary := fmt.Sprintf("Account has %d field(s) and %d record type(s) available to this connection", len(fields), len(recordTypes))
		return salesforce.RecordResult("Account", describe, summary), nil
	}

	// The overview lives on the sObject Basic Information resource
	// (/sobjects/Account), which is a different endpoint from /describe and
	// returns objectDescribe plus the current user's recently viewed accounts.
	resp, err := salesforce.ExecuteAPI(instanceURL, token, http.MethodGet, "/sobjects/Account", nil)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	if err := salesforce.CheckResponse(resp); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	var overview map[string]interface{}
	if err := json.Unmarshal(resp.Body, &overview); err != nil {
		return salesforce.ErrorResult(fmt.Sprintf("could not read the Salesforce response: %v", err)), nil
	}

	recent, _ := overview["recentItems"].([]interface{})
	label := "Account"
	if od, ok := overview["objectDescribe"].(map[string]interface{}); ok {
		if l, ok := od["label"].(string); ok && l != "" {
			label = l
		}
	}
	summary := fmt.Sprintf("Read the %s object setup and %d recently viewed account(s)", label, len(recent))
	return salesforce.RecordResult("Account", overview, summary), nil
}
