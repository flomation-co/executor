// Package list_voices lists all available ElevenLabs voices including
// pre-made, cloned, and professional voices accessible by the API key.
package list_voices

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	core "flomation.app/automate/executor"
	el "flomation.app/automate/executor/actions/elevenlabs"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "List Voices"
	Description  = "List all available ElevenLabs voices with their IDs, names, and characteristics"
	Website      = "https://www.flomation.co"
	Icon         = "microphone"
	Date         = "18/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "api_key",
		Type:        core.ConnectionTypeString,
		Label:       "ElevenLabs API Key",
		Placeholder: "sk_...",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "voices", Type: core.ConnectionTypeObject, Label: "Voices (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Voice Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey := el.OptionalString("api_key", inputs)
	if apiKey == "" {
		return errResult("api_key is required")
	}

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet,
		el.APIURL+"/voices", nil)
	if err != nil {
		return errResult(fmt.Sprintf("Failed to build request: %v", err))
	}
	req.Header.Set("xi-api-key", apiKey)

	client := &http.Client{Timeout: el.RequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return errResult(fmt.Sprintf("ElevenLabs API error: %v", err))
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, el.MaxResponseBody))

	if resp.StatusCode != http.StatusOK {
		return errResult(fmt.Sprintf("ElevenLabs API returned %d: %s", resp.StatusCode, string(body)))
	}

	var result struct {
		Voices []struct {
			VoiceID  string `json:"voice_id"`
			Name     string `json:"name"`
			Category string `json:"category"`
			Labels   map[string]string `json:"labels"`
			Preview  string `json:"preview_url"`
		} `json:"voices"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return errResult(fmt.Sprintf("Failed to parse response: %v", err))
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d voice(s):\n\n", len(result.Voices))
	var parsed []interface{}
	for _, v := range result.Voices {
		// Build label summary (accent, gender, age, etc.)
		var labelParts []string
		for k, val := range v.Labels {
			labelParts = append(labelParts, fmt.Sprintf("%s: %s", k, val))
		}
		labels := ""
		if len(labelParts) > 0 {
			labels = " (" + strings.Join(labelParts, ", ") + ")"
		}
		fmt.Fprintf(&sb, "• %s [%s]%s {id:%s}\n", v.Name, v.Category, labels, v.VoiceID)
		parsed = append(parsed, map[string]interface{}{
			"voice_id": v.VoiceID,
			"name":     v.Name,
			"category": v.Category,
			"labels":   v.Labels,
		})
	}

	return map[string]interface{}{
		"tool_result": sb.String(),
		"voices":      parsed,
		"count":       len(result.Voices),
		"success":     true,
		"error":       "",
	}, nil
}

func errResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result": msg,
		"voices":      nil,
		"count":       0,
		"success":     false,
		"error":       msg,
	}, nil
}