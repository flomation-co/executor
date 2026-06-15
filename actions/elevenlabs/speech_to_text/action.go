// Package speech_to_text transcribes audio to text using the ElevenLabs
// Speech-to-Text API. Accepts audio as base64-encoded data or a URL.
package speech_to_text

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	core "flomation.app/automate/executor"
	el "flomation.app/automate/executor/actions/elevenlabs"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Speech to Text"
	Description  = "Transcribe audio to text using ElevenLabs speech recognition"
	Website      = "https://www.flomation.co"
	Icon         = "microphone+file-lines"
	Date         = "18/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{
		Name:        "api_key",
		Type:        core.ConnectionTypeSecret,
		Label:       "ElevenLabs API Key",
		Placeholder: "sk_...",
		Required:    true,
	},
	{
		Name:        "audio_base64",
		Type:        core.ConnectionTypeSecret,
		Label:       "Audio data (base64-encoded). Provide this OR audio_url.",
		Placeholder: "base64 audio data",
	},
	{
		Name:        "audio_url",
		Type:        core.ConnectionTypeString,
		Label:       "URL to audio file. Provide this OR audio_base64.",
		Placeholder: "https://example.com/audio.mp3",
	},
	{
		Name:  "language_code",
		Type:  core.ConnectionTypeString,
		Label: "Language code (ISO 639-1, e.g. 'en', 'fr', 'de'). Leave empty for auto-detect.",
		Placeholder: "en",
	},
	{
		Name:  "model_id",
		Type:  core.ConnectionTypeString,
		Label: "Model",
		Options: []core.ConnectionOption{
			{Name: "Scribe v1 (default)", Value: "scribe_v1"},
		},
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Transcribed text"},
	{Name: "text", Type: core.ConnectionTypeString, Label: "Full transcription"},
	{Name: "language_code", Type: core.ConnectionTypeString, Label: "Detected language"},
	{Name: "words", Type: core.ConnectionTypeObject, Label: "Word-level timestamps (JSON)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey := el.OptionalString("api_key", inputs)
	if apiKey == "" {
		return errResult("api_key is required")
	}

	audioBase64 := el.OptionalString("audio_base64", inputs)
	audioURL := el.OptionalString("audio_url", inputs)
	if audioBase64 == "" && audioURL == "" {
		return errResult("Either audio_base64 or audio_url is required")
	}

	languageCode := el.OptionalString("language_code", inputs)
	modelID := el.OptionalString("model_id", inputs)
	if modelID == "" {
		modelID = "scribe_v1"
	}

	var audioData []byte
	var err error

	if audioURL != "" {
		// Download audio from URL
		audioData, err = downloadAudio(flow, audioURL)
		if err != nil {
			return errResult(fmt.Sprintf("Failed to download audio: %v", err))
		}
	} else {
		// Decode base64 audio
		audioData, err = base64.StdEncoding.DecodeString(audioBase64)
		if err != nil {
			// Try URL-safe base64
			audioData, err = base64.URLEncoding.DecodeString(audioBase64)
			if err != nil {
				return errResult(fmt.Sprintf("Failed to decode base64 audio: %v", err))
			}
		}
	}

	// Build multipart form request
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Detect audio format from content to set correct filename extension.
	// ElevenLabs uses the extension to infer the codec.
	filename := "audio.mp3"
	if len(audioData) >= 4 && string(audioData[0:4]) == "RIFF" {
		filename = "audio.wav"
	} else if len(audioData) >= 4 && string(audioData[0:4]) == "OggS" {
		filename = "audio.ogg"
	} else if len(audioData) >= 4 && string(audioData[0:4]) == "fLaC" {
		filename = "audio.flac"
	}

	// Add audio file
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return errResult(fmt.Sprintf("Failed to create form: %v", err))
	}
	if _, err := part.Write(audioData); err != nil {
		return errResult(fmt.Sprintf("Failed to write audio data: %v", err))
	}

	// Add model_id
	_ = writer.WriteField("model_id", modelID)

	// Add language code if provided
	if languageCode != "" {
		_ = writer.WriteField("language_code", languageCode)
	}

	if err := writer.Close(); err != nil {
		return errResult(fmt.Sprintf("Failed to close form: %v", err))
	}

	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost,
		el.APIURL+"/speech-to-text", &body)
	if err != nil {
		return errResult(fmt.Sprintf("Failed to build request: %v", err))
	}
	req.Header.Set("xi-api-key", apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: el.RequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return errResult(fmt.Sprintf("ElevenLabs API error: %v", err))
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, el.MaxResponseBody))

	if resp.StatusCode != http.StatusOK {
		return errResult(fmt.Sprintf("ElevenLabs API returned %d: %s", resp.StatusCode, string(respBody)))
	}

	var result struct {
		Text         string `json:"text"`
		LanguageCode string `json:"language_code"`
		Words        []struct {
			Text  string  `json:"text"`
			Start float64 `json:"start"`
			End   float64 `json:"end"`
			Type  string  `json:"type"`
		} `json:"words"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return errResult(fmt.Sprintf("Failed to parse response: %v", err))
	}

	text := strings.TrimSpace(result.Text)
	wordCount := len(strings.Fields(text))
	lang := result.LanguageCode
	if lang == "" {
		lang = "unknown"
	}

	return map[string]interface{}{
		"tool_result":   fmt.Sprintf("Transcribed %d words (language: %s):\n\n%s", wordCount, lang, text),
		"text":          text,
		"language_code": lang,
		"words":         result.Words,
		"success":       true,
		"error":         "",
	}, nil
}

func downloadAudio(flow *core.Flow, audioURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, audioURL, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: el.RequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d fetching audio", resp.StatusCode)
	}

	return io.ReadAll(io.LimitReader(resp.Body, el.MaxResponseBody))
}

func errResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result":   msg,
		"text":          "",
		"language_code": "",
		"words":         nil,
		"success":       false,
		"error":         msg,
	}, nil
}