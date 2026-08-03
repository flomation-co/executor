package linear_add_labels

import (
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	linear "flomation.app/automate/executor/actions/linear"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Add Labels"
	Description  = "Add labels (by name or UUID) to an issue without removing existing labels."
	Website      = "https://www.flomation.co"
	Icon         = "linear+tag"
	Date         = "03/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Linear API Key", Placeholder: "lin_api_...", Required: true},
	{Name: "issue_id", Type: core.ConnectionTypeString, Label: "Issue ID or Identifier", Placeholder: "UUID or FLO-123", Required: true},
	{Name: "labels", Type: core.ConnectionTypeString, Label: "Labels", Placeholder: "Comma-separated label names or UUIDs, e.g. Enriched, Priority", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Issue ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Issue"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := linear.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}
	issueRef, err := linear.RequiredString("issue_id", inputs)
	if err != nil {
		return errResult("an issue_id (UUID or identifier like FLO-123) is required"), nil
	}
	labelValues := linear.SplitCSV(linear.OptionalString("labels", inputs))
	if len(labelValues) == 0 {
		return errResult("at least one label (name or UUID) is required"), nil
	}

	// Resolve label names → UUIDs (UUIDs pass through).
	labelIDs, unresolved, err := linear.ResolveLabelIDs(apiKey, labelValues)
	if err != nil {
		return errResult(fmt.Sprintf("failed to resolve labels: %s", err)), nil
	}
	if len(unresolved) > 0 {
		return errResult(fmt.Sprintf("could not find label(s): %s — use list_labels to see available labels", strings.Join(unresolved, ", "))), nil
	}

	// Fetch the issue (id accepts a UUID or an identifier) with its current labels.
	resp, err := linear.ExecuteGraphQL(apiKey, linear.GraphQLRequest{
		Query: `query($id: String!) { issue(id: $id) { id identifier labels { nodes { id name } } } }`,
		Variables: map[string]interface{}{"id": issueRef},
	})
	if err != nil {
		return errResult(fmt.Sprintf("failed to load issue %q: %s", issueRef, err)), nil
	}
	var issueResp struct {
		Issue struct {
			ID         string `json:"id"`
			Identifier string `json:"identifier"`
			Labels     struct {
				Nodes []struct {
					ID string `json:"id"`
				} `json:"nodes"`
			} `json:"labels"`
		} `json:"issue"`
	}
	if err := json.Unmarshal(resp.Data, &issueResp); err != nil {
		return nil, fmt.Errorf("failed to parse issue: %w", err)
	}
	if issueResp.Issue.ID == "" {
		return errResult(fmt.Sprintf("issue %q not found", issueRef)), nil
	}

	// Merge existing + new label ids (union, preserving existing).
	seen := map[string]bool{}
	merged := make([]string, 0)
	for _, n := range issueResp.Issue.Labels.Nodes {
		if !seen[n.ID] {
			seen[n.ID] = true
			merged = append(merged, n.ID)
		}
	}
	added := 0
	for _, id := range labelIDs {
		if !seen[id] {
			seen[id] = true
			merged = append(merged, id)
			added++
		}
	}

	upd, err := linear.ExecuteGraphQL(apiKey, linear.GraphQLRequest{
		Query: `mutation($id: String!, $input: IssueUpdateInput!) {
			issueUpdate(id: $id, input: $input) {
				success
				issue { id identifier title labels { nodes { id name } } }
			}
		}`,
		Variables: map[string]interface{}{
			"id":    issueResp.Issue.ID,
			"input": map[string]interface{}{"labelIds": merged},
		},
	})
	if err != nil {
		return errResult(fmt.Sprintf("failed to update labels: %s", err)), nil
	}
	var updResp struct {
		IssueUpdate struct {
			Success bool            `json:"success"`
			Issue   json.RawMessage `json:"issue"`
		} `json:"issueUpdate"`
	}
	if err := json.Unmarshal(upd.Data, &updResp); err != nil {
		return nil, fmt.Errorf("failed to parse update: %w", err)
	}
	var issueObj interface{}
	_ = json.Unmarshal(updResp.IssueUpdate.Issue, &issueObj)

	summary := fmt.Sprintf("Added %d label(s) to %s (%d total).", added, issueResp.Issue.Identifier, len(merged))
	if added == 0 {
		summary = fmt.Sprintf("%s already had those label(s); no change.", issueResp.Issue.Identifier)
	}
	return map[string]interface{}{
		"tool_result": summary + "\n" + string(updResp.IssueUpdate.Issue),
		"id":          issueResp.Issue.ID,
		"result":      issueObj,
		"success":     updResp.IssueUpdate.Success,
		"error":       "",
	}, nil
}

func errResult(msg string) map[string]interface{} {
	return map[string]interface{}{"tool_result": msg, "success": false, "error": msg}
}
