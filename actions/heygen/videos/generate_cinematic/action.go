// Package generate_cinematic creates a HeyGen cinematic video: you describe the
// scene AND the action in a prompt and HeyGen generates the avatar performing it
// in that environment. This gives the most control over motion, gesture and
// setting — the strongest route to a presenter/streamer shot rather than a
// talking head. Asynchronous: poll Get Video with the returned video_id.
package generate_cinematic

import (
	"encoding/json"
	"strings"

	core "flomation.app/automate/executor"
	heygen "flomation.app/automate/executor/actions/heygen"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Generate Cinematic Video"
	Description  = "Generate a HeyGen video from a scene prompt — describe the setting and the avatar's motion/gestures."
	Website      = "https://www.flomation.co"
	Icon         = "film+star"
	Date         = "11/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "HeyGen API Key", Placeholder: "${secrets.HeyGenApiKey}", Required: true},
	{Name: "avatar_ids", Type: core.ConnectionTypeString, Label: "Avatar Look IDs", Placeholder: "1 to 3, comma-separated (from List Avatars)", Required: true},
	{Name: "prompt", Type: core.ConnectionTypeText, Label: "Scene & motion prompt", Placeholder: "Describe the setting and how the avatar moves/gestures", Required: true},
	{
		Name: "aspect_ratio", Type: core.ConnectionTypeString, Label: "Aspect Ratio",
		Options: []core.ConnectionOption{
			{Name: "16:9 (landscape)", Value: "16:9"},
			{Name: "9:16 (portrait)", Value: "9:16"},
			{Name: "1:1 (square)", Value: "1:1"},
		},
	},
	{
		Name: "resolution", Type: core.ConnectionTypeString, Label: "Resolution",
		Options: []core.ConnectionOption{
			{Name: "720p", Value: "720p"},
			{Name: "1080p", Value: "1080p"},
		},
	},
	{Name: "duration", Type: core.ConnectionTypeInteger, Label: "Duration (seconds, 4 to 15)"},
	{Name: "auto_duration", Type: core.ConnectionTypeBoolean, Label: "Auto duration (ignore Duration)"},
	{Name: "enhance_prompt", Type: core.ConnectionTypeBoolean, Label: "Let HeyGen enhance the prompt"},
	{Name: "references", Type: core.ConnectionTypeText, Label: "Advanced: reference assets JSON array (images/videos for style)"},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title"},
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
	prompt := heygen.OptionalString("prompt", inputs)
	if prompt == "" {
		return heygen.ErrorResult("a scene & motion prompt is required"), nil
	}
	avatarIDs := splitIDs(heygen.OptionalString("avatar_ids", inputs))
	if len(avatarIDs) == 0 {
		return heygen.ErrorResult("at least one avatar look id is required — pick from List Avatars"), nil
	}
	if len(avatarIDs) > 3 {
		return heygen.ErrorResult("cinematic videos accept at most 3 avatar look ids"), nil
	}

	body := map[string]interface{}{
		"type":      "cinematic_avatar",
		"prompt":    prompt,
		"avatar_id": avatarIDs,
	}
	heygen.SetString(body, "aspect_ratio", "aspect_ratio", inputs)
	heygen.SetString(body, "resolution", "resolution", inputs)
	if d := heygen.OptionalInt("duration", inputs); d != nil {
		body["duration"] = *d
	}
	if b := heygen.OptionalBool("auto_duration", inputs); b != nil {
		body["auto_duration"] = *b
	}
	if b := heygen.OptionalBool("enhance_prompt", inputs); b != nil {
		body["enhance_prompt"] = *b
	}
	if raw := heygen.OptionalString("references", inputs); raw != "" {
		var refs []interface{}
		if err := json.Unmarshal([]byte(raw), &refs); err != nil {
			return heygen.ErrorResult("references must be a JSON array"), nil
		}
		body["references"] = refs
	}
	heygen.SetString(body, "title", "title", inputs)
	heygen.SetString(body, "callback_url", "callback_url", inputs)
	heygen.SetString(body, "callback_id", "callback_id", inputs)

	resp, err := heygen.NewClient(apiKey).Post(flow, "/v3/videos", body)
	if err != nil {
		return heygen.MapError(err), nil
	}

	data := heygen.DataObj(resp)
	videoID := heygen.Str(data, "video_id")
	if videoID == "" {
		videoID = heygen.Str(data, "id")
	}
	status := heygen.Str(data, "status")
	if status == "" {
		status = "waiting"
	}

	return heygen.Result(
		"Cinematic video queued (id "+videoID+", status "+status+"). Poll Get Video, or wait for the webhook callback.",
		map[string]interface{}{"video_id": videoID, "status": status},
	), nil
}

// splitIDs turns a comma-separated list into a trimmed, blank-dropped slice.
func splitIDs(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
