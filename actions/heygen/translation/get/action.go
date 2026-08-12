// Package get retrieves the status and output URL of a HeyGen video translation
// job by its video_translate_id.
package get

import (
	"fmt"

	core "flomation.app/automate/executor"
	heygen "flomation.app/automate/executor/actions/heygen"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Get Translation"
	Description  = "Get the status and output URL of a HeyGen video translation job."
	Website      = "https://www.flomation.co"
	Icon         = "globe+magnifying-glass"
	Date         = "12/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "HeyGen API Key", Placeholder: "${secrets.HeyGenApiKey}", Required: true},
	{Name: "video_translate_id", Type: core.ConnectionTypeString, Label: "Translation ID", Placeholder: "From Translate Video", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "video_url", Type: core.ConnectionTypeString, Label: "Translated Video URL"},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title"},
	{Name: "message", Type: core.ConnectionTypeString, Label: "Message"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := heygen.GetAPIKey(inputs)
	if err != nil {
		return heygen.ErrorResult(err.Error()), nil
	}

	id := heygen.OptionalString("video_translate_id", inputs)
	if id == "" {
		return heygen.ErrorResult("video_translate_id is required"), nil
	}

	resp, err := heygen.NewClient(apiKey).Get(flow, "/v2/video_translate/"+id, nil)
	if err != nil {
		return heygen.MapError(err), nil
	}

	data := heygen.DataObj(resp)
	status := heygen.Str(data, "status")
	// HeyGen returns the output under "url"; expose it as video_url for parity
	// with Get Video.
	videoURL := heygen.Str(data, "url")
	title := heygen.Str(data, "title")
	message := heygen.Str(data, "message")

	var summary string
	switch {
	case videoURL != "":
		summary = fmt.Sprintf("Translation %s is %s: %s", id, status, videoURL)
	default:
		summary = fmt.Sprintf("Translation %s is %s.", id, status)
		if message != "" {
			summary += " " + message
		}
	}
	return heygen.Result(summary, map[string]interface{}{
		"status":    status,
		"video_url": videoURL,
		"title":     title,
		"message":   message,
	}), nil
}
