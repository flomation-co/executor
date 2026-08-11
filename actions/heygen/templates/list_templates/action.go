// Package list_templates lists the HeyGen Studio templates available to the
// account. A template bakes in the avatar, scene/background, layout and
// branding — use one to generate a polished presenter/streamer-style video by
// filling its variables rather than composing a plain talking-head.
package list_templates

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	heygen "flomation.app/automate/executor/actions/heygen"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "List Templates"
	Description  = "List available HeyGen Studio templates (avatar + scene + layout you can fill and render)."
	Website      = "https://www.flomation.co"
	Icon         = "copy+list"
	Date         = "11/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "HeyGen API Key", Placeholder: "${secrets.HeyGenApiKey}", Required: true},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Max results"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Templates"},
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
	if v := heygen.OptionalInt("limit", inputs); v != nil {
		q.Set("limit", fmt.Sprintf("%d", *v))
	}

	resp, err := heygen.NewClient(apiKey).Get(flow, "/v3/templates", q)
	if err != nil {
		return heygen.MapError(err), nil
	}

	tpls := heygen.ExtractList(resp, "templates", "list")
	return heygen.Result(fmt.Sprintf("Found %d template(s)", len(tpls)), map[string]interface{}{
		"results": tpls,
		"count":   int64(len(tpls)),
	}), nil
}
