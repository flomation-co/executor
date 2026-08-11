// Package list_avatars lists the HeyGen avatar looks available to the account.
// A look's id is the avatar_id you pass to Generate Avatar Video.
package list_avatars

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	heygen "flomation.app/automate/executor/actions/heygen"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "List Avatars"
	Description  = "List available HeyGen avatars (their look id is the avatar_id for generating a video)."
	Website      = "https://www.flomation.co"
	Icon         = "user+list"
	Date         = "11/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "HeyGen API Key", Placeholder: "${secrets.HeyGenApiKey}", Required: true},
	{Name: "group_id", Type: core.ConnectionTypeString, Label: "Avatar Group ID", Placeholder: "Optional: filter to one group"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Max results"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Avatars"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := heygen.GetAPIKey(inputs)
	if err != nil {
		return heygen.ErrorResult(err.Error()), nil
	}

	q := url.Values{}
	if v := heygen.OptionalString("group_id", inputs); v != "" {
		q.Set("group_id", v)
	}
	if v := heygen.OptionalInt("limit", inputs); v != nil {
		q.Set("limit", fmt.Sprintf("%d", *v))
	}

	resp, err := heygen.NewClient(apiKey).Get(flow, "/v3/avatars/looks", q)
	if err != nil {
		return heygen.MapError(err), nil
	}

	looks := heygen.ExtractList(resp, "looks", "avatars", "list")
	return heygen.Result(fmt.Sprintf("Found %d avatar(s)", len(looks)), map[string]interface{}{
		"results": looks,
		"count":   int64(len(looks)),
	}), nil
}
