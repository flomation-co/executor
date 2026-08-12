// Package create starts a HeyGen video translation (dubbing) job: take an
// existing video URL and re-voice it in another language. Asynchronous — returns
// a video_translate_id; poll Get Translation for status and the output URL.
package create

import (
	"fmt"

	core "flomation.app/automate/executor"
	heygen "flomation.app/automate/executor/actions/heygen"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Translate Video"
	Description  = "Translate/dub an existing video into another language with HeyGen."
	Website      = "https://www.flomation.co"
	Icon         = "globe+plus"
	Date         = "12/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "HeyGen API Key", Placeholder: "${secrets.HeyGenApiKey}", Required: true},
	{Name: "video_url", Type: core.ConnectionTypeString, Label: "Video URL", Placeholder: "A public URL of the source video to translate", Required: true},
	{Name: "output_language", Type: core.ConnectionTypeString, Label: "Output Language", Placeholder: "e.g. Spanish (see List Translation Languages)", Required: true},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title"},
	{Name: "translate_audio_only", Type: core.ConnectionTypeBoolean, Label: "Translate audio only (keep original video)"},
	{Name: "speaker_num", Type: core.ConnectionTypeInteger, Label: "Number of Speakers", Placeholder: "Optional: how many distinct speakers to detect"},
	{Name: "callback_id", Type: core.ConnectionTypeString, Label: "Callback ID", Placeholder: "Echoed back in the webhook"},
	{Name: "raw_json", Type: core.ConnectionTypeText, Label: "Advanced: extra JSON merged into the request body"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "video_translate_id", Type: core.ConnectionTypeString, Label: "Translation ID"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := heygen.GetAPIKey(inputs)
	if err != nil {
		return heygen.ErrorResult(err.Error()), nil
	}

	videoURL := heygen.OptionalString("video_url", inputs)
	outputLanguage := heygen.OptionalString("output_language", inputs)
	if videoURL == "" {
		return heygen.ErrorResult("video_url is required — the source video to translate"), nil
	}
	if outputLanguage == "" {
		return heygen.ErrorResult("output_language is required — pick one from List Translation Languages"), nil
	}

	body := map[string]interface{}{
		"video_url":       videoURL,
		"output_language": outputLanguage,
	}
	heygen.SetString(body, "title", "title", inputs)
	if c := core.FindConnection("translate_audio_only", inputs); c != nil && c.Boolean() != nil {
		body["translate_audio_only"] = *c.Boolean()
	}
	if n := heygen.OptionalInt("speaker_num", inputs); n != nil {
		body["speaker_num"] = *n
	}
	heygen.SetString(body, "callback_id", "callback_id", inputs)

	extra, perr := heygen.ParseJSONObject("raw_json", inputs)
	if perr != nil {
		return heygen.ErrorResult(perr.Error()), nil
	}
	for k, v := range extra {
		body[k] = v
	}

	resp, err := heygen.NewClient(apiKey).Post(flow, "/v2/video_translate", body)
	if err != nil {
		return heygen.MapError(err), nil
	}

	data := heygen.DataObj(resp)
	id := heygen.Str(data, "video_translate_id")
	status := heygen.Str(data, "status")
	if status == "" {
		status = "pending"
	}

	summary := fmt.Sprintf("Translation queued (id %s, status %s) into %s. Poll Get Translation with this id.", id, status, outputLanguage)
	return heygen.Result(summary, map[string]interface{}{
		"video_translate_id": id,
		"status":             status,
	}), nil
}
