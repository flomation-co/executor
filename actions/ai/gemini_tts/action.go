// Package gemini_tts implements the Gemini Text-to-Speech action —
// generate audio from a text prompt via Gemini's preview TTS model.
//
// Same generateContent endpoint as the text and image models, but the
// generationConfig carries a speechConfig (voice selection) and the
// response candidates come back with a single inline_data part
// containing PCM audio.
//
// Like the image action, the bytes are off-loaded to the BlobStore so
// downstream Slack/Telegram/Discord audio-upload actions can consume
// the flo:blob token without inflating any AI context window.
//
// AI-callable: tool_result is the first output and carries a short
// human-readable summary ("Generated audio: 'Hello, world' (Kore, 24 kB)")
// so an agent calling this as a tool gets readable feedback.
package gemini_tts

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Gemini Text-to-Speech"
	Description  = "Generate audio from a text prompt using Gemini's TTS preview"
	Website      = "https://www.flomation.co"
	Icon         = "brain+microphone"
	Date         = "25/06/2026"
	Type         = core.ActionTypeAction

	defaultModel    = "gemini-2.5-flash-preview-tts"
	defaultVoice    = "Kore"
	maxResponseBody = 32 << 20 // 32 MB
)

// apiBase is a var (not a const) so tests can point it at a
// httptest.Server URL. Production code never mutates this.
var apiBase = "https://generativelanguage.googleapis.com/v1beta/models/"

// Gemini's prebuilt voices. The full set is documented under
// gemini-2.5-flash-preview-tts; we expose the ones most useful for
// general-purpose narration. Users can type any voice name they want
// since the input is a free-text dropdown.
var Inputs = [...]core.Connection{
	{
		Name:        "api_key",
		Type:        core.ConnectionTypeSecret,
		Label:       "API Key",
		Placeholder: "AIza...",
		Required:    true,
	},
	{
		Name:  "model",
		Type:  core.ConnectionTypeSecret,
		Label: "Model",
		Options: []core.ConnectionOption{
			{Name: "Gemini 2.5 Flash Preview TTS", Value: "gemini-2.5-flash-preview-tts"},
			{Name: "Gemini 2.5 Pro Preview TTS", Value: "gemini-2.5-pro-preview-tts"},
		},
	},
	{
		Name:        "prompt",
		Type:        core.ConnectionTypeText,
		Label:       "Text",
		Placeholder: "Say cheerfully: hello, world!",
		Required:    true,
	},
	{
		Name:  "voice",
		Type:  core.ConnectionTypeSecret,
		Label: "Voice",
		Options: []core.ConnectionOption{
			{Name: "Kore (firm, default)", Value: "Kore"},
			{Name: "Puck (upbeat)", Value: "Puck"},
			{Name: "Charon (informative)", Value: "Charon"},
			{Name: "Fenrir (excitable)", Value: "Fenrir"},
			{Name: "Aoede (breezy)", Value: "Aoede"},
			{Name: "Leda (youthful)", Value: "Leda"},
			{Name: "Orus (firm, deeper)", Value: "Orus"},
			{Name: "Zephyr (bright)", Value: "Zephyr"},
		},
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Tool Result"},
	{Name: "audio_blob", Type: core.ConnectionTypeString, Label: "Audio (flo:blob token)"},
	{Name: "audio_base64", Type: core.ConnectionTypeString, Label: "Audio (base64)"},
	{Name: "mime_type", Type: core.ConnectionTypeString, Label: "MIME Type"},
	{Name: "voice", Type: core.ConnectionTypeString, Label: "Voice Used"},
	{Name: "model", Type: core.ConnectionTypeString, Label: "Model Used"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKeyConn := core.FindConnection("api_key", inputs)
	if apiKeyConn == nil || apiKeyConn.String() == nil || *apiKeyConn.String() == "" {
		return errorResult("api_key is required"), nil
	}
	apiKey := *apiKeyConn.String()

	promptConn := core.FindConnection("prompt", inputs)
	if promptConn == nil || promptConn.String() == nil || strings.TrimSpace(*promptConn.String()) == "" {
		return errorResult("prompt is required"), nil
	}
	prompt := *promptConn.String()

	model := defaultModel
	if c := core.FindConnection("model", inputs); c != nil && c.String() != nil && *c.String() != "" {
		model = *c.String()
	}

	voice := defaultVoice
	if c := core.FindConnection("voice", inputs); c != nil && c.String() != nil && *c.String() != "" {
		voice = *c.String()
	}

	payload := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": prompt}}},
		},
		"generationConfig": map[string]interface{}{
			"responseModalities": []string{"AUDIO"},
			"speechConfig": map[string]interface{}{
				"voiceConfig": map[string]interface{}{
					"prebuiltVoiceConfig": map[string]interface{}{
						"voiceName": voice,
					},
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to marshal request: %v", err)), nil
	}

	url := apiBase + model + ":generateContent"
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return errorResult(fmt.Sprintf("failed to create request: %v", err)), nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", apiKey)

	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return errorResult(fmt.Sprintf("Gemini request failed: %v", err)), nil
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))

	if resp.StatusCode != http.StatusOK {
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(respBody, &apiErr)
		msg := apiErr.Error.Message
		if msg == "" {
			msg = string(respBody)
		}
		return errorResult(fmt.Sprintf("Gemini API error (%d): %s", resp.StatusCode, msg)), nil
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					InlineData *struct {
						MimeType string `json:"mimeType"`
						Data     string `json:"data"`
					} `json:"inlineData,omitempty"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		ModelVersion string `json:"modelVersion"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return errorResult(fmt.Sprintf("failed to parse Gemini response: %v", err)), nil
	}
	if len(result.Candidates) == 0 {
		return errorResult("Gemini returned no candidates"), nil
	}

	var audioB64, mimeType string
	for _, p := range result.Candidates[0].Content.Parts {
		if p.InlineData != nil && p.InlineData.Data != "" {
			audioB64 = p.InlineData.Data
			mimeType = p.InlineData.MimeType
			// Gemini's TTS returns raw PCM as "audio/L16;codec=pcm;
			// rate=24000" — keep the full mime so downstream players
			// know the rate and encoding rather than assuming a
			// container.
			break
		}
	}
	if audioB64 == "" {
		return errorResult("Gemini did not return audio"), nil
	}

	audioBytes, err := base64.StdEncoding.DecodeString(audioB64)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to decode audio bytes: %v", err)), nil
	}

	var blobToken string
	if blobs := flow.Blobs(); blobs != nil {
		token, err := blobs.Put(audioBytes, mimeType)
		if err == nil {
			blobToken = token
		}
	}

	resolvedModel := result.ModelVersion
	if resolvedModel == "" {
		resolvedModel = model
	}

	// tool_result preview is the first ~80 chars of the input text so an
	// AI agent calling this as a tool gets a self-describing summary
	// without having to re-read its own prompt.
	preview := prompt
	if len(preview) > 80 {
		preview = preview[:77] + "..."
	}
	toolResult := fmt.Sprintf("Generated audio: %q (%s, %s, %d bytes)", preview, voice, mimeType, len(audioBytes))

	return map[string]interface{}{
		"tool_result":  toolResult,
		"audio_blob":   blobToken,
		"audio_base64": audioB64,
		"mime_type":    mimeType,
		"voice":        voice,
		"model":        resolvedModel,
		"success":      true,
		"error":        "",
	}, nil
}

func errorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result":  "TTS failed: " + msg,
		"audio_blob":   "",
		"audio_base64": "",
		"mime_type":    "",
		"voice":        "",
		"model":        "",
		"success":      false,
		"error":        msg,
	}
}
