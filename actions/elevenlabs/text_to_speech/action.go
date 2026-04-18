// Package text_to_speech converts text to spoken audio using the ElevenLabs
// TTS API. Returns the audio as base64-encoded data with the selected format.
package text_to_speech

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	core "flomation.app/automate/executor"
	el "flomation.app/automate/executor/actions/elevenlabs"
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Text to Speech"
	Description  = "Convert text to spoken audio using ElevenLabs AI voices"
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
	{
		Name:        "text",
		Type:        core.ConnectionTypeText,
		Label:       "Text to convert to speech",
		Placeholder: "Hello, welcome to Flomation!",
		Required:    true,
	},
	{
		Name:        "voice_id",
		Type:        core.ConnectionTypeString,
		Label:       "Voice ID (from list_voices, or use a name like 'Rachel', 'Adam')",
		Placeholder: "21m00Tcm4TlvDq8ikWAM",
		Required:    true,
	},
	{
		Name:  "model_id",
		Type:  core.ConnectionTypeString,
		Label: "Model",
		Options: []core.ConnectionOption{
			{Name: "Multilingual v2 (best quality)", Value: "eleven_multilingual_v2"},
			{Name: "Turbo v2.5 (low latency)", Value: "eleven_turbo_v2_5"},
			{Name: "Turbo v2 (low latency)", Value: "eleven_turbo_v2"},
			{Name: "English v1", Value: "eleven_monolingual_v1"},
		},
	},
	{
		Name:  "stability",
		Type:  core.ConnectionTypeString,
		Label: "Stability (0.0-1.0, higher = more consistent, lower = more expressive)",
		Placeholder: "0.5",
	},
	{
		Name:  "similarity_boost",
		Type:  core.ConnectionTypeString,
		Label: "Similarity Boost (0.0-1.0, higher = closer to original voice)",
		Placeholder: "0.75",
	},
	{
		Name:  "style",
		Type:  core.ConnectionTypeString,
		Label: "Style exaggeration (0.0-1.0, v2 models only)",
		Placeholder: "0.0",
	},
	{
		Name:  "use_speaker_boost",
		Type:  core.ConnectionTypeBoolean,
		Label: "Use Speaker Boost (enhances voice clarity and presence)",
	},
	{
		Name:  "output_format",
		Type:  core.ConnectionTypeString,
		Label: "Output Format",
		Options: []core.ConnectionOption{
			{Name: "MP3 (44.1kHz, 128kbps)", Value: "mp3_44100_128"},
			{Name: "MP3 (44.1kHz, 64kbps)", Value: "mp3_44100_64"},
			{Name: "MP3 (22.05kHz, 32kbps)", Value: "mp3_22050_32"},
			{Name: "PCM (16kHz)", Value: "pcm_16000"},
			{Name: "PCM (22.05kHz)", Value: "pcm_22050"},
			{Name: "PCM (24kHz)", Value: "pcm_24000"},
			{Name: "PCM (44.1kHz)", Value: "pcm_44100"},
			{Name: "u-law (8kHz, telephony)", Value: "ulaw_8000"},
		},
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "audio_base64", Type: core.ConnectionTypeString, Label: "Audio (base64)"},
	{Name: "audio_format", Type: core.ConnectionTypeString, Label: "Audio Format"},
	{Name: "audio_size_bytes", Type: core.ConnectionTypeInteger, Label: "Audio Size (bytes)"},
	{Name: "character_count", Type: core.ConnectionTypeInteger, Label: "Characters Used"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey := el.OptionalString("api_key", inputs)
	if apiKey == "" {
		return errResult("api_key is required")
	}

	text := el.OptionalString("text", inputs)
	if text == "" {
		return errResult("text is required")
	}

	voiceID := el.OptionalString("voice_id", inputs)
	if voiceID == "" {
		return errResult("voice_id is required — use elevenlabs/list_voices to find available voices")
	}

	modelID := el.OptionalString("model_id", inputs)
	if modelID == "" {
		modelID = "eleven_multilingual_v2"
	}

	outputFormat := el.OptionalString("output_format", inputs)
	if outputFormat == "" {
		outputFormat = "mp3_44100_128"
	}

	// Build request body
	reqBody := map[string]interface{}{
		"text":     text,
		"model_id": modelID,
	}

	// Voice settings (optional)
	voiceSettings := map[string]interface{}{}
	if v := el.OptionalString("stability", inputs); v != "" {
		var f float64
		fmt.Sscanf(v, "%f", &f)
		voiceSettings["stability"] = f
	}
	if v := el.OptionalString("similarity_boost", inputs); v != "" {
		var f float64
		fmt.Sscanf(v, "%f", &f)
		voiceSettings["similarity_boost"] = f
	}
	if v := el.OptionalString("style", inputs); v != "" {
		var f float64
		fmt.Sscanf(v, "%f", &f)
		voiceSettings["style"] = f
	}
	if boostConn := core.FindConnection("use_speaker_boost", inputs); boostConn != nil {
		if b := boostConn.Boolean(); b != nil {
			voiceSettings["use_speaker_boost"] = *b
		}
	}
	if len(voiceSettings) > 0 {
		reqBody["voice_settings"] = voiceSettings
	}

	payload, _ := json.Marshal(reqBody)

	endpoint := fmt.Sprintf("%s/text-to-speech/%s?output_format=%s",
		el.APIURL, voiceID, outputFormat)

	// Use a bounded context for the entire TTS request (including body read).
	// ElevenLabs can stream audio slowly, so the flow's Go context alone
	// may not have a deadline. This ensures we never hang indefinitely.
	reqCtx, reqCancel := context.WithTimeout(flow.GoContext(), el.RequestTimeout)
	defer reqCancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost,
		endpoint, bytes.NewReader(payload))
	if err != nil {
		return errResult(fmt.Sprintf("Failed to build request: %v", err))
	}
	req.Header.Set("xi-api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "audio/mpeg")

	client := &http.Client{Timeout: el.RequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		if reqCtx.Err() != nil {
			return errResult(fmt.Sprintf("ElevenLabs TTS timed out after %v", el.RequestTimeout))
		}
		return errResult(fmt.Sprintf("ElevenLabs API error: %v", err))
	}
	defer func() { _ = resp.Body.Close() }()

	log.WithFields(log.Fields{
		"status":         resp.StatusCode,
		"content_length": resp.ContentLength,
	}).Info("[elevenlabs_tts] response received, reading body")

	audioBytes, err := io.ReadAll(io.LimitReader(resp.Body, el.MaxResponseBody))
	if err != nil {
		if reqCtx.Err() != nil {
			return errResult(fmt.Sprintf("ElevenLabs TTS body read timed out after %v", el.RequestTimeout))
		}
		return errResult(fmt.Sprintf("Failed to read audio response: %v", err))
	}

	log.WithFields(log.Fields{
		"size_bytes": len(audioBytes),
	}).Info("[elevenlabs_tts] body read complete")

	if resp.StatusCode != http.StatusOK {
		return errResult(fmt.Sprintf("ElevenLabs API returned %d: %s", resp.StatusCode, string(audioBytes)))
	}

	audioBase64 := base64.StdEncoding.EncodeToString(audioBytes)
	charCount := len([]rune(text))

	return map[string]interface{}{
		"tool_result":      fmt.Sprintf("Generated %d bytes of %s audio from %d characters of text", len(audioBytes), outputFormat, charCount),
		"audio_base64":     audioBase64,
		"audio_format":     outputFormat,
		"audio_size_bytes": len(audioBytes),
		"character_count":  charCount,
		"success":          true,
		"error":            "",
	}, nil
}

func errResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result":      msg,
		"audio_base64":     "",
		"audio_format":     "",
		"audio_size_bytes": 0,
		"character_count":  0,
		"success":          false,
		"error":            msg,
	}, nil
}