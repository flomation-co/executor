// Package crm_salesforce_quick_action_get_all lists the quick actions an admin
// has configured — the buttons Salesforce users click ("New Contact", "Log a
// Call") — so an automation can run one by name instead of writing the record
// itself and guessing at record types, defaults and required fields.
package crm_salesforce_quick_action_get_all

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
	Name         = "Salesforce: Get Many Quick Actions"
	Description  = "List the quick actions your Salesforce administrator has set up, either across the whole org or on one type of record."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+clipboard-list"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "object", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "Account — leave blank for the org-wide quick actions"},
	{Name: "name_contains", Type: core.ConnectionTypeString, Label: "Name Contains", Placeholder: "call — leave blank to list them all"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Most quick actions to return — leave blank for all of them"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Quick Actions"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total_size", Type: core.ConnectionTypeInteger, Label: "Total Available"},
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

	// Two different endpoints, one action: quick actions are either global (the
	// "+" menu anywhere) or attached to one object. Blank means global, which
	// is the safe default — an operator who has not chosen a record type is
	// browsing, not targeting.
	path := "/quickActions"
	scope := "org-wide"
	if object := salesforce.OptionalString("object", inputs); object != "" {
		obj, err := salesforce.ValidateSOQLObjectName(object)
		if err != nil {
			return nil, err
		}
		path = "/sobjects/" + obj + "/quickActions"
		scope = obj
	}

	resp, err := salesforce.ExecuteAPI(instanceURL, token, http.MethodGet, path, nil)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	if err := salesforce.CheckResponse(resp); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// This endpoint answers with a bare JSON ARRAY rather than the usual
	// {records: [...]} envelope, so it is decoded here instead of going through
	// the shared query helpers.
	var actions []map[string]interface{}
	if err := json.Unmarshal(resp.Body, &actions); err != nil {
		return salesforce.ErrorResult(fmt.Sprintf("failed to parse the Salesforce quick action list: %v", err)), nil
	}

	// Like the flow catalogue, this endpoint has no pagination — everything the
	// connected user can see comes back in one response — so the search and the
	// limit are applied client-side and next_url is always empty.
	matched := filterByName(actions, salesforce.OptionalString("name_contains", inputs))
	total := len(matched)

	limit, limitSet := salesforce.OptionalInt("limit", inputs)
	if limitSet && limit > 0 && limit < len(matched) {
		matched = matched[:limit]
	}

	summary := fmt.Sprintf("Found %d %s quick action(s)", len(matched), scope)
	if len(matched) < total {
		summary = fmt.Sprintf("Showing %d of %d %s quick action(s)", len(matched), total, scope)
	}
	if total == 0 {
		summary = fmt.Sprintf("No %s quick actions found — quick actions are set up per object in Salesforce Setup, so try a different record type", scope)
	}
	return salesforce.ListResult(matched, "", total, summary), nil
}

// filterByName keeps the quick actions whose API name or label contains the
// operator's search text, case-insensitively. Both are matched because the
// label ("Log a Call") is what the operator recognises while the API name
// ("LogACall") is what Run Quick Action needs.
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
