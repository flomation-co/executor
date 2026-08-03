package linear_get_current_user

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	linear "flomation.app/automate/executor/actions/linear"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Get Current User"
	Description  = "Get the user the API key belongs to (id, name, email) — use for self-assignment."
	Website      = "https://www.flomation.co"
	Icon         = "linear+user"
	Date         = "03/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Linear API Key", Placeholder: "lin_api_...", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "User ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "User"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := linear.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}

	resp, err := linear.ExecuteGraphQL(apiKey, linear.GraphQLRequest{
		Query: `query { viewer { id name displayName email } }`,
	})
	if err != nil {
		return map[string]interface{}{"tool_result": fmt.Sprintf("Failed: %s", err), "success": false, "error": err.Error()}, nil
	}

	var out struct {
		Viewer struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			DisplayName string `json:"displayName"`
			Email       string `json:"email"`
		} `json:"viewer"`
	}
	if err := json.Unmarshal(resp.Data, &out); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	var generic interface{}
	_ = json.Unmarshal(resp.Data, &generic)
	if m, ok := generic.(map[string]interface{}); ok {
		generic = m["viewer"]
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("You are %s (%s) {id:%s}", out.Viewer.Name, out.Viewer.Email, out.Viewer.ID),
		"id":          out.Viewer.ID,
		"result":      generic,
		"success":     true,
		"error":       "",
	}, nil
}
