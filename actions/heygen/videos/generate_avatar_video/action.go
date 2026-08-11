// Package generate_avatar_video creates a HeyGen avatar (talking-head) video
// from a script or an audio track. Generation is asynchronous: this returns a
// video_id with status "waiting" — poll Get Video, or pass a callback_url and
// use the HeyGen webhook trigger to continue when it is ready.
package generate_avatar_video

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	heygen "flomation.app/automate/executor/actions/heygen"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Generate Avatar Video"
	Description  = "Create a HeyGen talking-head video from a script (text-to-speech) or an audio URL."
	Website      = "https://www.flomation.co"
	Icon         = "film+plus"
	Date         = "11/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "HeyGen API Key", Placeholder: "${secrets.HeyGenApiKey}", Required: true},
	{Name: "avatar_id", Type: core.ConnectionTypeString, Label: "Avatar ID", Placeholder: "From List Avatars", Required: true},
	{Name: "script", Type: core.ConnectionTypeText, Label: "Script (spoken text)", Placeholder: "What the avatar should say (uses text-to-speech)"},
	{Name: "voice_id", Type: core.ConnectionTypeString, Label: "Voice ID", Placeholder: "Optional: blank uses the avatar's default voice"},
	{Name: "audio_url", Type: core.ConnectionTypeString, Label: "Audio URL", Placeholder: "Alternative to a script: lip-sync to this audio"},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title"},
	{
		Name: "aspect_ratio", Type: core.ConnectionTypeString, Label: "Aspect Ratio",
		Options: []core.ConnectionOption{
			{Name: "Auto", Value: "auto"},
			{Name: "16:9 (landscape)", Value: "16:9"},
			{Name: "9:16 (portrait)", Value: "9:16"},
			{Name: "1:1 (square)", Value: "1:1"},
			{Name: "4:5", Value: "4:5"},
			{Name: "5:4", Value: "5:4"},
		},
	},
	{
		Name: "resolution", Type: core.ConnectionTypeString, Label: "Resolution",
		Options: []core.ConnectionOption{
			{Name: "1080p", Value: "1080p"},
			{Name: "720p", Value: "720p"},
			{Name: "4K", Value: "4k"},
		},
	},
	{Name: "voice_speed", Type: core.ConnectionTypeString, Label: "Voice Speed", Placeholder: "0.5 to 1.5 (default 1.0)"},
	{Name: "background_image_url", Type: core.ConnectionTypeString, Label: "Background Image URL", Placeholder: "A room/studio image to place the avatar in"},
	{Name: "background_color", Type: core.ConnectionTypeString, Label: "Background Colour", Placeholder: "#150e14 (ignored if a background image is set)"},
	{Name: "remove_background", Type: core.ConnectionTypeBoolean, Label: "Remove avatar background (needs a matting-trained avatar)"},
	{
		Name: "fit", Type: core.ConnectionTypeString, Label: "Fit",
		Options: []core.ConnectionOption{
			{Name: "Cover (fill, may crop)", Value: "cover"},
			{Name: "Contain (fit, may letterbox)", Value: "contain"},
		},
	},
	{Name: "motion_prompt", Type: core.ConnectionTypeString, Label: "Motion Prompt", Placeholder: "e.g. gesture naturally and glance at the screen"},
	{
		Name: "expressiveness", Type: core.ConnectionTypeString, Label: "Expressiveness (photo/Avatar IV)",
		Options: []core.ConnectionOption{
			{Name: "High", Value: "high"},
			{Name: "Medium", Value: "medium"},
			{Name: "Low", Value: "low"},
		},
	},
	{
		Name: "engine", Type: core.ConnectionTypeString, Label: "Avatar Engine",
		Options: []core.ConnectionOption{
			{Name: "Default", Value: ""},
			{Name: "Avatar III", Value: "avatar_iii"},
			{Name: "Avatar IV", Value: "avatar_iv"},
			{Name: "Avatar V (most dynamic)", Value: "avatar_v"},
		},
	},
	{Name: "callback_url", Type: core.ConnectionTypeString, Label: "Webhook Callback URL", Placeholder: "Called when the video is ready"},
	{Name: "callback_id", Type: core.ConnectionTypeString, Label: "Callback ID", Placeholder: "Echoed back in the webhook"},
	{Name: "raw_json", Type: core.ConnectionTypeText, Label: "Advanced: extra JSON merged into the request body"},
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

	avatarID := heygen.OptionalString("avatar_id", inputs)
	if avatarID == "" {
		return heygen.ErrorResult("avatar_id is required — pick one from List Avatars"), nil
	}
	script := heygen.OptionalString("script", inputs)
	audioURL := heygen.OptionalString("audio_url", inputs)
	if script == "" && audioURL == "" {
		return heygen.ErrorResult("provide a script (with a voice_id) or an audio_url for the avatar to speak"), nil
	}

	body := map[string]interface{}{
		"type":      "avatar",
		"avatar_id": avatarID,
	}
	if script != "" {
		body["script"] = script
		// voice_id is optional: when omitted, HeyGen speaks with the avatar's
		// configured default voice (set in the HeyGen dashboard — which may be an
		// ElevenLabs voice). Only send it when the author picks a specific voice.
		heygen.SetString(body, "voice_id", "voice_id", inputs)
		if speed := heygen.OptionalFloat("voice_speed", inputs); speed != nil {
			body["voice_settings"] = map[string]interface{}{"speed": *speed}
		}
	} else {
		body["audio_url"] = audioURL
	}

	heygen.SetString(body, "title", "title", inputs)
	heygen.SetString(body, "aspect_ratio", "aspect_ratio", inputs)
	heygen.SetString(body, "resolution", "resolution", inputs)

	// Presenter/scene settings: place the avatar in a room, matte it out, and
	// make it more dynamic — the levers that turn a talking head into a presenter.
	if img := heygen.OptionalString("background_image_url", inputs); img != "" {
		body["background"] = map[string]interface{}{"type": "image", "url": img}
	} else if col := heygen.OptionalString("background_color", inputs); col != "" {
		body["background"] = map[string]interface{}{"type": "color", "value": col}
	}
	if c := core.FindConnection("remove_background", inputs); c != nil && c.Boolean() != nil {
		body["remove_background"] = *c.Boolean()
	}
	heygen.SetString(body, "fit", "fit", inputs)
	heygen.SetString(body, "motion_prompt", "motion_prompt", inputs)
	heygen.SetString(body, "expressiveness", "expressiveness", inputs)
	if eng := heygen.OptionalString("engine", inputs); eng != "" {
		body["engine"] = map[string]interface{}{"type": eng}
	}

	heygen.SetString(body, "callback_url", "callback_url", inputs)
	heygen.SetString(body, "callback_id", "callback_id", inputs)

	// Advanced override.
	extra, perr := heygen.ParseJSONObject("raw_json", inputs)
	if perr != nil {
		return heygen.ErrorResult(perr.Error()), nil
	}
	for k, v := range extra {
		body[k] = v
	}

	// HeyGen wants `engine` as an object ({"type":"avatar_v"}). Agents and the
	// raw_json path often pass it as a plain string ("avatar_v"), which HeyGen
	// rejects with "Input should be a valid dictionary or object". Coerce it so
	// either form works, from any source.
	normalizeEngine(body)
	normalizeBackground(body)
	defaultPortraitFit(body)

	resp, err := heygen.NewClient(apiKey).Post(flow, "/v3/videos", body)
	if err != nil {
		return heygen.MapError(err), nil
	}

	data := heygen.DataObj(resp)
	videoID := heygen.Str(data, "video_id")
	status := heygen.Str(data, "status")
	if status == "" {
		status = "waiting"
	}

	summary := fmt.Sprintf("Video queued (id %s, status %s). Poll Get Video with this id, or wait for the webhook callback.", videoID, status)
	return heygen.Result(summary, map[string]interface{}{
		"video_id": videoID,
		"status":   status,
	}), nil
}

// normalizeEngine coerces a string engine ("avatar_v") into the object HeyGen
// requires ({"type":"avatar_v"}), and drops an empty engine entirely. A value
// already in object form is left untouched.
func normalizeEngine(body map[string]interface{}) {
	switch v := body["engine"].(type) {
	case string:
		if v == "" {
			delete(body, "engine")
		} else {
			body["engine"] = map[string]interface{}{"type": v}
		}
	case nil:
		delete(body, "engine")
	}
}

// normalizeBackground coerces a string background into the object HeyGen
// requires: a leading "#" (or a bare hex value) becomes {"type":"color",
// "value":...}, an http(s) URL becomes {"type":"image","url":...}. An empty
// string is dropped; an object is left untouched.
func normalizeBackground(body map[string]interface{}) {
	s, ok := body["background"].(string)
	if !ok {
		return
	}
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		delete(body, "background")
	case strings.HasPrefix(s, "http://"), strings.HasPrefix(s, "https://"):
		body["background"] = map[string]interface{}{"type": "image", "url": s}
	default:
		// Treat anything else (e.g. "#150e14" or "150e14") as a colour value.
		if !strings.HasPrefix(s, "#") && isHex(s) {
			s = "#" + s
		}
		body["background"] = map[string]interface{}{"type": "color", "value": s}
	}
}

func isHex(s string) bool {
	if len(s) != 3 && len(s) != 6 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

// defaultPortraitFit sets fit=cover for a portrait canvas (9:16 / 4:5) when the
// author hasn't chosen a fit. HeyGen otherwise letterboxes a landscape avatar
// into a vertical frame (white bars top/bottom); cover fills it. Runs after the
// raw_json merge so it sees the final aspect_ratio and respects any explicit fit.
func defaultPortraitFit(body map[string]interface{}) {
	if _, hasFit := body["fit"]; hasFit {
		return
	}
	switch ar, _ := body["aspect_ratio"].(string); ar {
	case "9:16", "4:5":
		body["fit"] = "cover"
	}
}
