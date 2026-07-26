// Package crm_salesforce_flow_get_all lists the autolaunched Salesforce flows
// an operator can run, so "Run Flow" can be pointed at one by name without
// anyone opening Setup to look the API name up.
package crm_salesforce_flow_get_all

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Many Flows"
	Description  = "List the Salesforce flows that can be run from an automation, with the exact name to use in Run Flow."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+list"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "name_contains", Type: core.ConnectionTypeString, Label: "Name Contains", Placeholder: "welcome — leave blank to list every flow"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Most flows to return — leave blank for all of them"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Flows"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total_size", Type: core.ConnectionTypeInteger, Label: "Total Available"},
	{Name: "next_url", Type: core.ConnectionTypeString, Label: "Next Page URL"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Raw Response"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// flowListResponse is the Invocable Actions catalogue envelope. Each entry is
// {label, name, type, url} — "name" is the API name Run Flow needs, "label" is
// the friendly name the admin sees in Setup, and the two are rarely the same.
type flowListResponse struct {
	Actions []map[string]interface{} `json:"actions"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	resp, err := salesforce.ExecuteAPI(instanceURL, token, http.MethodGet, "/actions/custom/flow", nil)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	if err := salesforce.CheckResponse(resp); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	var listed flowListResponse
	if err := json.Unmarshal(resp.Body, &listed); err != nil {
		return salesforce.ErrorResult(fmt.Sprintf("failed to parse the Salesforce flow list: %v", err)), nil
	}

	// This endpoint has no pagination at all — Salesforce returns the whole
	// catalogue in one response — so there is deliberately no "Return All"
	// toggle here and next_url is always empty. Filtering and the limit are
	// applied client-side, which costs nothing extra: the call is the same size
	// either way, the trimming just keeps the flow's output readable.
	matched := filterByName(listed.Actions, salesforce.OptionalString("name_contains", inputs))
	total := len(matched)

	limit, limitSet := salesforce.OptionalInt("limit", inputs)
	if limitSet && limit > 0 && limit < len(matched) {
		matched = matched[:limit]
	}

	summary := fmt.Sprintf("Found %d flow(s)", len(matched))
	if len(matched) < total {
		summary = fmt.Sprintf("Showing %d of %d flow(s)", len(matched), total)
	}
	if total == 0 {
		summary = "No flows found — only ACTIVE autolaunched flows can be run from an automation; screen flows and record-triggered flows never appear here"
	}
	return salesforce.ListResult(matched, "", total, summary), nil
}

// filterByName keeps the flows whose API name or label contains the operator's
// search text, case-insensitively. Both are matched because an operator
// searching for "welcome" is thinking of the label they see in Salesforce,
// while the API name they will eventually paste is Send_Welcome_Email.
func filterByName(actions []map[string]interface{}, contains string) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(actions))
	needle := strings.ToLower(strings.TrimSpace(contains))
	for _, a := range actions {
		if needle == "" {
			out = append(out, a)
			continue
		}
		name, _ := a["name"].(string)
		label, _ := a["label"].(string)
		if strings.Contains(strings.ToLower(name), needle) || strings.Contains(strings.ToLower(label), needle) {
			out = append(out, a)
		}
	}
	return out
}
