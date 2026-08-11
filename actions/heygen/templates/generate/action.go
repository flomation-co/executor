// Package generate renders a video from a HeyGen Studio template by filling its
// variables. The template supplies the avatar, scene/background, layout and
// branding — so this produces a polished presenter/streamer-style video, not a
// plain talking head. Generation is asynchronous: poll Get Video with the
// returned video_id, or pass a callback_url for the webhook.
package generate

import (
	"fmt"

	core "flomation.app/automate/executor"
	heygen "flomation.app/automate/executor/actions/heygen"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Generate from Template"
	Description  = "Render a video from a HeyGen template by filling its variables (avatar, scene and branding are baked in)."
	Website      = "https://www.flomation.co"
	Icon         = "copy+plus"
	Date         = "11/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "HeyGen API Key", Placeholder: "${secrets.HeyGenApiKey}", Required: true},
	{Name: "template_id", Type: core.ConnectionTypeString, Label: "Template ID", Required: true},
	{Name: "text_values", Type: core.ConnectionTypeText, Label: "Text variables: JSON object of {\"variable\":\"value\"} (auto-typed as text)"},
	{Name: "variables", Type: core.ConnectionTypeText, Label: "Advanced: full typed variables JSON (from Get Template) — overrides text_values"},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title"},
	{Name: "caption", Type: core.ConnectionTypeBoolean, Label: "Burn in captions"},
	{Name: "callback_url", Type: core.ConnectionTypeString, Label: "Webhook Callback URL", Placeholder: "Called when the video is ready"},
	{Name: "callback_id", Type: core.ConnectionTypeString, Label: "Callback ID", Placeholder: "Echoed back in the webhook"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "video_id", Type: core.ConnectionTypeString, Label: "Video ID"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := heygen.GetAPIKey(inputs)
	if err != nil {
		return heygen.ErrorResult(err.Error()), nil
	}
	templateID := heygen.OptionalString("template_id", inputs)
	if templateID == "" {
		return heygen.ErrorResult("template_id is required"), nil
	}

	textValues, err := heygen.ParseJSONObject("text_values", inputs)
	if err != nil {
		return heygen.ErrorResult(err.Error()), nil
	}
	fullVars, err := heygen.ParseJSONObject("variables", inputs)
	if err != nil {
		return heygen.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{}
	if vars := buildVariables(textValues, fullVars); len(vars) > 0 {
		body["variables"] = vars
	}
	heygen.SetString(body, "title", "title", inputs)
	if c := heygen.OptionalBool("caption", inputs); c != nil {
		body["caption"] = *c
	}
	heygen.SetString(body, "callback_url", "callback_url", inputs)
	heygen.SetString(body, "callback_id", "callback_id", inputs)

	resp, err := heygen.NewClient(apiKey).Post(flow, "/v3/templates/"+templateID+"/generate", body)
	if err != nil {
		return heygen.MapError(err), nil
	}

	data := heygen.DataObj(resp)
	videoID := firstNonEmpty(heygen.Str(data, "video_id"), heygen.Str(data, "id"))
	status := heygen.Str(data, "status")
	if status == "" {
		status = "waiting"
	}

	return heygen.Result(
		fmt.Sprintf("Template video queued (id %s, status %s). Poll Get Video, or wait for the webhook callback.", videoID, status),
		map[string]interface{}{"video_id": videoID, "status": status},
	), nil
}

// buildVariables merges the convenience text_values (each wrapped as a HeyGen
// text variable) with the advanced full-typed variables, which take precedence.
func buildVariables(textValues, fullVars map[string]interface{}) map[string]interface{} {
	vars := map[string]interface{}{}
	for k, v := range textValues {
		vars[k] = map[string]interface{}{"type": "text", "content": fmt.Sprintf("%v", v)}
	}
	for k, v := range fullVars {
		vars[k] = v
	}
	return vars
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
