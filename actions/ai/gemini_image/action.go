// Package gemini_image implements the Gemini Image action — generate
// an image from a text prompt via Google's Gemini 2.5 Flash Image model
// (the "Nano Banana" image-gen API).
//
// Unlike the Gemini Prompt action, this one isn't part of any tool
// loop. It takes a prompt, hits :generateContent on the image model,
// pulls the inline_data part out of the response, off-loads the bytes
// to the executor's BlobStore, and emits a flo:blob token alongside
// the base64. Downstream actions (Slack file_upload, Telegram
// sendPhoto, Google Drive upload, etc.) consume the token via the
// engine's existing detokenisation path so the image bytes never
// inflate the AI's context window.
//
// AI-callable: the first output is tool_result (a one-line human
// description like "Generated image: a cat in a hat (size, mime)")
// so an AI agent calling this as a tool gets readable feedback.
package gemini_image

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
	Name         = "Gemini Image"
	Description  = "Generate an image from a text prompt using Gemini 2.5 Flash Image"
	Website      = "https://www.flomation.co"
	Icon         = "brain+image"
	Date         = "25/06/2026"
	Type         = core.ActionTypeAction

	defaultModel    = "gemini-2.5-flash-image"
	maxResponseBody = 32 << 20 // 32 MB — image responses can be a few MB each
)

// apiBase is a var (not a const) so tests can point it at a
// httptest.Server URL. Production code never mutates this.
var apiBase = "https://generativelanguage.googleapis.com/v1beta/models/"

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
			{Name: "Gemini 2.5 Flash Image (Nano Banana)", Value: "gemini-2.5-flash-image"},
			{Name: "Gemini 2.0 Flash Preview Image Generation", Value: "gemini-2.0-flash-preview-image-generation"},
		},
	},
	{
		Name:        "prompt",
		Type:        core.ConnectionTypeText,
		Label:       "Prompt",
		Placeholder: "A photorealistic image of a cat wearing a top hat, studio lighting.",
		Required:    true,
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Tool Result"},
	{Name: "image_blob", Type: core.ConnectionTypeString, Label: "Image (flo:blob token)"},
	{Name: "image_base64", Type: core.ConnectionTypeString, Label: "Image (base64)"},
	{Name: "mime_type", Type: core.ConnectionTypeString, Label: "MIME Type"},
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

	// Gemini image generation request: same generateContent endpoint as
	// the text model, with the image model in the URL path. The
	// response_modalities setting tells Gemini we expect an image back
	// (and optionally text) — without it the model defaults to text-
	// only output and won't produce inline_data parts.
	payload := map[string]interface{}{
		"contents": []map[string]interface{}{
			{"role": "user", "parts": []map[string]interface{}{{"text": prompt}}},
		},
		"generationConfig": map[string]interface{}{
			"responseModalities": []string{"IMAGE", "TEXT"},
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

	// Image generation can take 30s+ on the larger Imagen variants. Use
	// a wider budget than the text action.
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
					Text       *string `json:"text,omitempty"`
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

	// Walk the parts and pick out the first inline_data — that's our
	// image. Any text part is captured separately and surfaced as the
	// tool_result description.
	var imageB64, mimeType, textCaption string
	for _, p := range result.Candidates[0].Content.Parts {
		if p.InlineData != nil && p.InlineData.Data != "" {
			if imageB64 == "" {
				imageB64 = p.InlineData.Data
				mimeType = p.InlineData.MimeType
				if mimeType == "" {
					mimeType = "image/png"
				}
			}
		}
		if p.Text != nil && *p.Text != "" {
			textCaption = strings.TrimSpace(*p.Text)
		}
	}

	if imageB64 == "" {
		// Surface any text Gemini did return (it sometimes refuses with
		// a safety message instead of producing the image) so the user
		// knows WHY no image came back.
		reason := "Gemini did not return an image"
		if textCaption != "" {
			reason = reason + ": " + textCaption
		}
		return errorResult(reason), nil
	}

	imageBytes, err := base64.StdEncoding.DecodeString(imageB64)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to decode image bytes: %v", err)), nil
	}

	// Off-load to the blob store so downstream actions can consume the
	// image via flo:blob:<handle> instead of carrying the base64
	// through every node's output. Token resolution is automatic on
	// the way into any downstream action.
	var blobToken string
	if blobs := flow.Blobs(); blobs != nil {
		token, err := blobs.Put(imageBytes, mimeType)
		if err == nil {
			blobToken = token
		}
	}

	resolvedModel := result.ModelVersion
	if resolvedModel == "" {
		resolvedModel = model
	}

	// tool_result is one short human/AI-readable line. The base64 is
	// available on the dedicated output for nodes that want it
	// directly; the blob token is what most downstream chain steps
	// will use.
	toolResult := fmt.Sprintf("Generated image (%s, %d bytes)", mimeType, len(imageBytes))
	if textCaption != "" {
		toolResult = textCaption + " — " + toolResult
	}

	return map[string]interface{}{
		"tool_result":  toolResult,
		"image_blob":   blobToken,
		"image_base64": imageB64,
		"mime_type":    mimeType,
		"model":        resolvedModel,
		"success":      true,
		"error":        "",
	}, nil
}

// errorResult returns the canonical failure-shape output map. Used by
// every early-exit path so an AI agent calling this as a tool sees a
// consistent tool_result + success=false instead of a Go-side error
// (which would fail the whole node rather than surfacing the reason).
func errorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result":  "Image generation failed: " + msg,
		"image_blob":   "",
		"image_base64": "",
		"mime_type":    "",
		"model":        "",
		"success":      false,
		"error":        msg,
	}
}
