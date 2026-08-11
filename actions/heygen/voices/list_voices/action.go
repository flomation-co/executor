// Package list_voices lists the HeyGen voices available to the account. A
// voice's id is the voice_id you pass to Generate Avatar Video when using a
// script.
package list_voices

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	heygen "flomation.app/automate/executor/actions/heygen"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "List Voices"
	Description  = "List available HeyGen voices (their id is the voice_id for text-to-speech generation)."
	Website      = "https://www.flomation.co"
	Icon         = "microphone+list"
	Date         = "11/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "HeyGen API Key", Placeholder: "${secrets.HeyGenApiKey}", Required: true},
	{Name: "language", Type: core.ConnectionTypeString, Label: "Language", Placeholder: "Optional: filter, e.g. English"},
	{Name: "gender", Type: core.ConnectionTypeString, Label: "Gender", Placeholder: "Optional: male / female"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Max results"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Voices"},
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
	if v := heygen.OptionalString("language", inputs); v != "" {
		q.Set("language", v)
	}
	if v := heygen.OptionalString("gender", inputs); v != "" {
		q.Set("gender", v)
	}
	if v := heygen.OptionalInt("limit", inputs); v != nil {
		q.Set("limit", fmt.Sprintf("%d", *v))
	}

	resp, err := heygen.NewClient(apiKey).Get(flow, "/v3/voices", q)
	if err != nil {
		return heygen.MapError(err), nil
	}

	voices := heygen.ExtractList(resp, "voices", "list")
	return heygen.Result(fmt.Sprintf("Found %d voice(s)", len(voices)), map[string]interface{}{
		"results": voices,
		"count":   int64(len(voices)),
	}), nil
}
