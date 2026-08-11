// Package get_video fetches the status of a HeyGen video and, once ready, its
// download URL. Use it to poll after Generate Avatar Video.
package get_video

import (
	"fmt"

	core "flomation.app/automate/executor"
	heygen "flomation.app/automate/executor/actions/heygen"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Get Video"
	Description  = "Get a HeyGen video's status and, when ready, its download URL."
	Website      = "https://www.flomation.co"
	Icon         = "film+circle-info"
	Date         = "11/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "HeyGen API Key", Placeholder: "${secrets.HeyGenApiKey}", Required: true},
	{Name: "video_id", Type: core.ConnectionTypeString, Label: "Video ID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "video_url", Type: core.ConnectionTypeString, Label: "Video URL"},
	{Name: "thumbnail_url", Type: core.ConnectionTypeString, Label: "Thumbnail URL"},
	{Name: "subtitle_url", Type: core.ConnectionTypeString, Label: "Subtitle URL"},
	{Name: "duration", Type: core.ConnectionTypeString, Label: "Duration (seconds)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := heygen.GetAPIKey(inputs)
	if err != nil {
		return heygen.ErrorResult(err.Error()), nil
	}
	videoID := heygen.OptionalString("video_id", inputs)
	if videoID == "" {
		return heygen.ErrorResult("video_id is required"), nil
	}

	resp, err := heygen.NewClient(apiKey).Get(flow, "/v3/videos/"+videoID, nil)
	if err != nil {
		return heygen.MapError(err), nil
	}

	data := heygen.DataObj(resp)
	status := heygen.Str(data, "status")
	// HeyGen has used both video_url and video_url_caption across versions; take
	// the first non-empty. Subtitles and thumbnail arrive with the finished video.
	videoURL := firstNonEmpty(heygen.Str(data, "video_url"), heygen.Str(data, "url"))
	subtitleURL := firstNonEmpty(heygen.Str(data, "subtitle_url"), heygen.Str(data, "caption_url"))
	thumbURL := heygen.Str(data, "thumbnail_url")
	duration := ""
	if data != nil {
		if d, ok := data["duration"].(float64); ok {
			duration = fmt.Sprintf("%g", d)
		} else {
			duration = heygen.Str(data, "duration")
		}
	}

	var summary string
	switch status {
	case "completed", "success":
		summary = fmt.Sprintf("Video %s is ready: %s", videoID, videoURL)
	case "failed", "error":
		summary = fmt.Sprintf("Video %s failed to render.", videoID)
	default:
		summary = fmt.Sprintf("Video %s status: %s (not ready yet).", videoID, status)
	}

	return heygen.Result(summary, map[string]interface{}{
		"status":        status,
		"video_url":     videoURL,
		"thumbnail_url": thumbURL,
		"subtitle_url":  subtitleURL,
		"duration":      duration,
	}), nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
