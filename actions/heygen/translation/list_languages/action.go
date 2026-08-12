// Package list_languages lists the target languages HeyGen can translate a
// video into. A language name here is what you pass as output_language to
// Translate Video.
package list_languages

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	heygen "flomation.app/automate/executor/actions/heygen"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "List Translation Languages"
	Description  = "List the target languages HeyGen can translate/dub a video into."
	Website      = "https://www.flomation.co"
	Icon         = "globe+list"
	Date         = "12/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "HeyGen API Key", Placeholder: "${secrets.HeyGenApiKey}", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Languages"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := heygen.GetAPIKey(inputs)
	if err != nil {
		return heygen.ErrorResult(err.Error()), nil
	}

	resp, err := heygen.NewClient(apiKey).Get(flow, "/v2/video_translate/target_languages", nil)
	if err != nil {
		return heygen.MapError(err), nil
	}

	// Response shape: {"data":{"languages":["English","Spanish",...]}}.
	languages := stringList(heygen.DataObj(resp)["languages"])

	summary := fmt.Sprintf("HeyGen can translate into %d language(s)", len(languages))
	if len(languages) > 0 {
		preview := languages
		if len(preview) > 12 {
			preview = preview[:12]
		}
		summary += ": " + strings.Join(preview, ", ")
		if len(languages) > len(preview) {
			summary += ", …"
		}
	}
	return heygen.Result(summary, map[string]interface{}{
		"results": languages,
		"count":   int64(len(languages)),
	}), nil
}

// stringList coerces an interface{} holding a JSON array of strings into a
// []string, ignoring any non-string entries.
func stringList(v interface{}) []string {
	arr, ok := v.([]interface{})
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
