package crm_salesforce_task_describe

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
	Name         = "Salesforce: Get Task Object Info"
	Description  = "Return what your Salesforce org's Task object looks like, plus the tasks you last viewed. Handy for checking which status, priority and subject values your org actually uses, and as a quick test that the connection works."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+magnifying-glass"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "include_fields", Type: core.ConnectionTypeBoolean, Label: "Include Every Field and Its Options"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Task ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Task Object Info"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	// Two different Salesforce resources sit behind one checkbox. The default is
	// the cheap one — /sobjects/Task returns a summary of the object plus the
	// caller's recently viewed tasks, which is what makes it a good connection
	// test. Ticking the box switches to the full describe, a much larger payload
	// listing every field and every picklist value, so it is opt-in rather than
	// something a flow pays for on every run.
	if salesforce.OptionalBool("include_fields", inputs) {
		describe, err := salesforce.DescribeObject(instanceURL, token, "Task")
		if err != nil {
			return salesforce.ErrorResult(err.Error()), nil
		}
		fields, _ := describe["fields"].([]interface{})
		summary := fmt.Sprintf("Task object: %d field(s), including every picklist's options", len(fields))
		return salesforce.RecordResult("", describe, summary), nil
	}

	resp, err := salesforce.ExecuteAPI(instanceURL, token, http.MethodGet, "/sobjects/Task", nil)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	if err := salesforce.CheckResponse(resp); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	var info map[string]interface{}
	if err := json.Unmarshal(resp.Body, &info); err != nil {
		return salesforce.ErrorResult(fmt.Sprintf("failed to parse Salesforce response: %s", err.Error())), nil
	}

	label := "Task"
	if describe, ok := info["objectDescribe"].(map[string]interface{}); ok {
		if l, ok := describe["label"].(string); ok && l != "" {
			label = l
		}
	}
	recent, _ := info["recentItems"].([]interface{})
	summary := fmt.Sprintf("%s object info, with %d recently viewed task(s)", label, len(recent))
	return salesforce.RecordResult("", info, summary), nil
}
