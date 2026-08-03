package linear_list_projects

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
	Name         = "List Projects"
	Description  = "List projects with their UUIDs. Use this to get the project UUID needed by create/update_issue."
	Website      = "https://www.flomation.co"
	Icon         = "linear+list"
	Date         = "03/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Linear API Key", Placeholder: "lin_api_...", Required: true},
	{Name: "query", Type: core.ConnectionTypeString, Label: "Name Filter (optional)", Placeholder: "Filter projects by name substring"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "projects", Type: core.ConnectionTypeObject, Label: "Projects"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := linear.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}
	filter := strings.ToLower(strings.TrimSpace(linear.OptionalString("query", inputs)))

	resp, err := linear.ExecuteGraphQL(apiKey, linear.GraphQLRequest{
		Query: `query { projects(first: 250) { nodes { id name state progress } } }`,
	})
	if err != nil {
		return map[string]interface{}{"tool_result": fmt.Sprintf("Failed: %s", err), "success": false, "error": err.Error()}, nil
	}

	var out struct {
		Projects struct {
			Nodes []json.RawMessage `json:"nodes"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(resp.Data, &out); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	type row struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		State string `json:"state"`
	}
	var sb strings.Builder
	var parsed []interface{}
	shown := 0
	for _, raw := range out.Projects.Nodes {
		var r row
		if err := json.Unmarshal(raw, &r); err != nil {
			continue
		}
		if filter != "" && !strings.Contains(strings.ToLower(r.Name), filter) {
			continue
		}
		fmt.Fprintf(&sb, "• %s (%s) {id:%s}\n", r.Name, r.State, r.ID)
		var generic interface{}
		_ = json.Unmarshal(raw, &generic)
		parsed = append(parsed, generic)
		shown++
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d project(s):\n\n%s", shown, sb.String()),
		"projects":    parsed,
		"count":       shown,
		"success":     true,
		"error":       "",
	}, nil
}
