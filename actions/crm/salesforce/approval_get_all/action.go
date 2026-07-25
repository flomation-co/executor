// Package crm_salesforce_approval_get_all lists the approval processes an org
// has configured, and optionally what is currently sitting in them.
//
// Approvals are the human-in-the-loop step in a Salesforce org — "a discount
// over 20% goes to the manager" — and the two questions an automation needs
// answered before it can take part are "which approval processes exist, and
// what are they called" and "what is waiting, and on whom". This action answers
// both, because they come from two completely different places:
//
//   - The process definitions come from GET /process/approvals/, a purpose-built
//     endpoint that returns them grouped by sObject.
//   - What is actually pending comes from SOQL over ProcessInstanceWorkitem,
//     which the approvals endpoint knows nothing about.
//
// Pairing them here means an operator picking an approval process by name for
// the Submit action, or building a "chase the outstanding approvals" digest,
// does not have to know that distinction exists.
package crm_salesforce_approval_get_all

import (
	"encoding/json"
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
	Name         = "Salesforce: Get Many Approval Processes"
	Description  = "List the approval processes set up in your Salesforce org, and optionally the records currently waiting for approval in each."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+clipboard-list"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

// pendingItemFields is the SELECT list for the outstanding work items. The
// relationship hops matter: a work item on its own says almost nothing, whereas
// ProcessInstance.TargetObjectId names the record awaiting approval and
// ProcessInstance.ProcessDefinition.Name names the process it is stuck in —
// which is exactly what a chase-up message needs to say.
const pendingItemFields = "Id, ActorId, OriginalActorId, CreatedDate, ProcessInstanceId, ProcessInstance.TargetObjectId, ProcessInstance.Status, ProcessInstance.ProcessDefinitionId, ProcessInstance.ProcessDefinition.Name"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "object", Type: core.ConnectionTypeString, Label: "Object", Placeholder: "Opportunity — leave empty for every object"},
	{Name: "include_pending_items", Type: core.ConnectionTypeBoolean, Label: "Also List What Is Waiting for Approval"},
	{Name: "pending_for_user_id", Type: core.ConnectionTypeString, Label: "Waiting On (User)", Placeholder: "0055f00000AbcDEAA — only items assigned to this person"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit (items waiting)", Placeholder: "50 by default, up to 2000 — applies to the waiting items, not the processes"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Approval Processes"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total_size", Type: core.ConnectionTypeInteger, Label: "Records Returned"},
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

	objectFilter := salesforce.OptionalString("object", inputs)
	if objectFilter != "" {
		if _, err := salesforce.ValidateSOQLObjectName(objectFilter); err != nil {
			return nil, fmt.Errorf("Object: %w", err)
		}
	}
	actorID := salesforce.OptionalString("pending_for_user_id", inputs)
	if actorID != "" {
		if err := salesforce.ValidateRecordID(actorID); err != nil {
			return nil, fmt.Errorf("Waiting On (User): %w", err)
		}
	}

	// The trailing slash is Salesforce's, not a typo: /process/approvals
	// without it is a different (404) resource on some org configurations.
	resp, err := salesforce.ExecuteAPI(instanceURL, token, http.MethodGet, "/process/approvals/", nil)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	if err := salesforce.CheckResponse(resp); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	processes, err := parseApprovalProcesses(resp.Body, objectFilter)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	pending := []map[string]interface{}{}
	includePending := salesforce.OptionalBool("include_pending_items", inputs)
	// Naming the person the items are waiting on is an unambiguous request to
	// see those items. Without this, filling in Waiting On and leaving the
	// checkbox alone returns the process definitions with no pending
	// information attached at all and nothing to say why — the filter would be
	// a silent no-op.
	if actorID != "" {
		includePending = true
	}
	if includePending {
		limit, limitSet := salesforce.OptionalInt("limit", inputs)
		pending, err = fetchPendingItems(instanceURL, token, actorID, salesforce.ClampLimit(limit, limitSet))
		if err != nil {
			return salesforce.ErrorResult(err.Error()), nil
		}
		attachPendingItems(processes, pending)
	}

	// There is no paging on this endpoint — Salesforce returns every process
	// definition in one response — so the cursor is always empty and the total
	// is simply what came back.
	out := salesforce.ListResult(processes, "", len(processes), summarise(processes, len(pending), includePending, objectFilter))
	if raw, ok := out["result"].(map[string]interface{}); ok {
		// The full pending list also goes on the raw response, so items whose
		// process was filtered out (or is no longer active) are still reachable.
		raw["pendingItems"] = toInterfaceSlice(pending)
		raw["pendingCount"] = len(pending)
	}
	return out, nil
}

// approvalProcess is one entry of the approvals endpoint's response. Salesforce
// groups them by sObject name, so the object is both the map key and a field on
// each entry.
type approvalProcess struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Object      string `json:"object"`
	Description string `json:"description"`
	SortOrder   int    `json:"sortOrder"`
}

// parseApprovalProcesses flattens the response's object-keyed map into one
// stably-ordered list.
//
// The response shape is {"approvals":{"Account":[{...}],"Opportunity":[{...}]}}.
// A map is fine for a machine and useless for an operator scrolling a list, so
// it is flattened and sorted by object, then by the org's own sort order, then
// by name — a stable order matters because this list feeds a picker and a
// reshuffling dropdown is maddening.
func parseApprovalProcesses(body []byte, objectFilter string) ([]map[string]interface{}, error) {
	var envelope struct {
		Approvals map[string][]approvalProcess `json:"approvals"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("failed to parse the Salesforce approvals response: %w", err)
	}

	out := []map[string]interface{}{}
	for object, defs := range envelope.Approvals {
		if objectFilter != "" && !strings.EqualFold(object, objectFilter) {
			continue
		}
		for _, def := range defs {
			// The object is echoed on each entry, but not by every org — fall
			// back to the map key it was filed under.
			objectName := def.Object
			if objectName == "" {
				objectName = object
			}
			out = append(out, map[string]interface{}{
				"id":          def.ID,
				"name":        def.Name,
				"object":      objectName,
				"description": def.Description,
				"sortOrder":   def.SortOrder,
			})
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		oi, _ := out[i]["object"].(string)
		oj, _ := out[j]["object"].(string)
		if oi != oj {
			return oi < oj
		}
		si, _ := out[i]["sortOrder"].(int)
		sj, _ := out[j]["sortOrder"].(int)
		if si != sj {
			return si < sj
		}
		ni, _ := out[i]["name"].(string)
		nj, _ := out[j]["name"].(string)
		return ni < nj
	})
	return out, nil
}

// fetchPendingItems queries the outstanding approval work items.
//
// Worth knowing before an operator reports this as a bug: ProcessInstanceWorkitem
// is permission-filtered like any other object, so a connected user without
// View All Data sees only the items assigned to them. An empty list from an
// admin means nothing is pending; an empty list from a standard user means
// nothing is pending FOR THEM.
func fetchPendingItems(instanceURL, token, actorID string, limit int) ([]map[string]interface{}, error) {
	conditions := []salesforce.Condition{}
	if actorID != "" {
		conditions = append(conditions, salesforce.Condition{Field: "ActorId", Operator: "=", Value: actorID})
	}
	soql, err := salesforce.BuildQuery("ProcessInstanceWorkitem", pendingItemFields, conditions, false, "CreatedDate DESC", limit, true)
	if err != nil {
		return nil, err
	}
	records, _, _, _, err := salesforce.Query(instanceURL, token, soql, false, false)
	if err != nil {
		return nil, err
	}
	return records, nil
}

// attachPendingItems files each work item under the process it belongs to, so a
// flow can loop the processes and read what is stuck in each without doing the
// join itself. The approvals endpoint's id and ProcessInstance.ProcessDefinitionId
// are the same ProcessDefinition ID, which is what makes the join possible.
func attachPendingItems(processes []map[string]interface{}, pending []map[string]interface{}) {
	byDefinition := map[string][]interface{}{}
	for _, item := range pending {
		instance, _ := item["ProcessInstance"].(map[string]interface{})
		if instance == nil {
			continue
		}
		defID := salesforce.StringifyID(instance["ProcessDefinitionId"])
		if defID == "" {
			continue
		}
		byDefinition[defID] = append(byDefinition[defID], item)
	}
	for _, process := range processes {
		id, _ := process["id"].(string)
		items := byDefinition[id]
		if items == nil {
			// Non-nil so a process with nothing waiting serialises as [] rather
			// than null — a Loop node over it should simply do nothing.
			items = []interface{}{}
		}
		process["pendingItems"] = items
		process["pendingCount"] = len(items)
	}
}

// toInterfaceSlice widens records for JSON output, keeping an empty list as []
// rather than null.
func toInterfaceSlice(records []map[string]interface{}) []interface{} {
	out := make([]interface{}, 0, len(records))
	for _, r := range records {
		out = append(out, r)
	}
	return out
}

// summarise renders the operator-facing one-liner.
//
// The waiting-items count is the number ATTACHED to the processes being
// returned, not the number the work-item query found. Those differ whenever an
// Object filter is set — the work items are fetched org-wide and then joined —
// and reporting the org-wide figure next to a filtered list of processes reads
// as "47 Opportunities are waiting" when three are. The remainder is still
// reachable on result.pendingItems, so the summary says where it went.
func summarise(processes []map[string]interface{}, pendingTotal int, includePending bool, objectFilter string) string {
	scope := ""
	if objectFilter != "" {
		scope = " for " + objectFilter
	}
	summary := fmt.Sprintf("Found %d approval process(es)%s", len(processes), scope)
	if !includePending {
		return summary
	}
	attached := 0
	for _, process := range processes {
		if n, ok := process["pendingCount"].(int); ok {
			attached += n
		}
	}
	summary += fmt.Sprintf(", with %d record(s) waiting for approval", attached)
	if pendingTotal > attached {
		summary += fmt.Sprintf(" (%d more waiting on other processes — see result.pendingItems)", pendingTotal-attached)
	}
	return summary
}
