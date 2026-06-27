// Package gemini_video implements the Gemini Video action — generate
// a short video clip from a text prompt (optionally seeded with an
// input image) via Google's Veo 2 and Veo 3 models.
//
// Unlike :generateContent (used by gemini text/image), the Veo API is
// asynchronous. The request returns a Long-Running Operation handle;
// the action then polls that operation every few seconds until it
// completes, then downloads the rendered MP4 bytes from the operation
// response. From the flow author's perspective this is still one
// synchronous node — the polling is hidden inside Execute. Typical
// completion time is 30–90s; we cap the wait at 5 minutes to stay
// inside the runner's per-node budget.
//
// AI-callable: the first output is tool_result (a one-line human
// description) so an AI agent calling this as a tool gets readable
// feedback. The video bytes are off-loaded to the executor's
// BlobStore — agents see only the flo:blob token in tool_result, not
// the multi-megabyte base64, which would otherwise blow the context
// window.
package gemini_video

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
	log "github.com/sirupsen/logrus"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Gemini Video"
	Description  = "Generate a short video clip from a text prompt using Google's Veo 2 or Veo 3"
	Website      = "https://www.flomation.co"
	Icon         = "brain+video"
	Date         = "27/06/2026"
	Type         = core.ActionTypeAction

	// Default to Veo 3.1 Fast — cheapest of the current preview
	// family, supports natively-synced audio. Veo 2 is still listed
	// in the dropdown as a fallback because some accounts may not
	// have the 3.1 preview rolled out yet, but Google's docs treat
	// veo-3.0-* as deprecated and don't currently expose any
	// "stable" Veo 3 identifier, so the safest default for a fresh
	// API key is Fast preview.
	defaultModel = "veo-3.1-fast-generate-preview"

	// Response payload ceiling — Veo videos are usually 1–5 MB, but
	// the response also carries metadata. 64 MB is comfortably above
	// the worst observed clip.
	maxResponseBody = 64 << 20
)

// apiBase, pollInterval, pollTimeout are vars (not consts) so tests
// can point apiBase at a httptest.Server URL and dial down the poll
// cadence to keep the suite fast. Production code never mutates them.
//
// Polling budget rationale: Veo typically completes in 30–90s. We
// poll at 5s intervals and bail at 5 minutes to stay well inside the
// runner's per-node timeout (~5 min). Tightening these isn't useful
// — the LRO doesn't transition faster than the model finishes
// rendering; loosening them risks the runner killing the executor
// mid-poll, orphaning a paid-for generation.
var (
	apiBase      = "https://generativelanguage.googleapis.com/v1beta/"
	pollInterval = 5 * time.Second
	pollTimeout  = 5 * time.Minute
)

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
			// Order = recommended preference. Veo 3.1 family ships
			// in preview today (Jun 2026); Veo 2 is the legacy
			// fallback for accounts where preview hasn't rolled out.
			// Google's model identifiers move fast — if a value
			// stops working, hit ListModels to discover the current
			// set.
			{Name: "Veo 3.1 Fast (preview, with audio, cheapest)", Value: "veo-3.1-fast-generate-preview"},
			{Name: "Veo 3.1 (preview, with audio, best quality)", Value: "veo-3.1-generate-preview"},
			{Name: "Veo 3.1 Lite (preview, with audio, lowest cost)", Value: "veo-3.1-lite-generate-preview"},
			{Name: "Veo 2 (legacy, video only)", Value: "veo-2.0-generate-001"},
		},
	},
	{
		Name:        "prompt",
		Type:        core.ConnectionTypeText,
		Label:       "Prompt",
		Placeholder: "Photorealistic flythrough over snowy Welsh mountains at golden hour, drone footage.",
		Required:    true,
	},
	{
		Name:        "input_image_blob",
		Type:        core.ConnectionTypeText,
		Label:       "Input Image (flo:blob token, optional — enables image-to-video)",
		Placeholder: "flo:blob:abc...",
	},
	{
		Name:  "aspect_ratio",
		Type:  core.ConnectionTypeText,
		Label: "Aspect Ratio",
		Options: []core.ConnectionOption{
			{Name: "16:9 (landscape)", Value: "16:9"},
			{Name: "9:16 (portrait)", Value: "9:16"},
			{Name: "1:1 (square)", Value: "1:1"},
		},
	},
	{
		Name:        "duration_seconds",
		Type:        core.ConnectionTypeInteger,
		Label:       "Duration (seconds, 5–8 — model-dependent)",
		Placeholder: "8",
	},
	{
		Name:        "negative_prompt",
		Type:        core.ConnectionTypeText,
		Label:       "Negative Prompt (things to avoid, optional)",
		Placeholder: "blurry, low quality",
	},
	{
		Name:  "person_generation",
		Type:  core.ConnectionTypeText,
		Label: "Person Generation",
		Options: []core.ConnectionOption{
			{Name: "Allow adults (default)", Value: "allow_adult"},
			{Name: "Don't allow people", Value: "dont_allow"},
		},
	},
	{
		Name:        "seed",
		Type:        core.ConnectionTypeInteger,
		Label:       "Seed (optional, for reproducibility)",
		Placeholder: "12345",
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Tool Result"},
	{Name: "video_blob", Type: core.ConnectionTypeString, Label: "Video (flo:blob token)"},
	{Name: "video_base64", Type: core.ConnectionTypeString, Label: "Video (base64)"},
	{Name: "mime_type", Type: core.ConnectionTypeString, Label: "MIME Type"},
	{Name: "model", Type: core.ConnectionTypeString, Label: "Model Used"},
	{Name: "operation_name", Type: core.ConnectionTypeString, Label: "Operation Name (LRO handle)"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKeyConn := core.FindConnection("api_key", inputs)
	if apiKeyConn == nil || apiKeyConn.String() == nil || *apiKeyConn.String() == "" {
		return errorResult("api_key is required", ""), nil
	}
	apiKey := *apiKeyConn.String()

	promptConn := core.FindConnection("prompt", inputs)
	if promptConn == nil || promptConn.String() == nil || strings.TrimSpace(*promptConn.String()) == "" {
		return errorResult("prompt is required", ""), nil
	}
	prompt := *promptConn.String()

	model := defaultModel
	if c := core.FindConnection("model", inputs); c != nil && c.String() != nil && *c.String() != "" {
		model = *c.String()
	}

	// Optional generation parameters. Empty/zero values mean "let
	// the model use its default" — we only emit the parameter when
	// the user has explicitly chosen something.
	aspectRatio := ""
	if c := core.FindConnection("aspect_ratio", inputs); c != nil && c.String() != nil {
		aspectRatio = *c.String()
	}
	negativePrompt := ""
	if c := core.FindConnection("negative_prompt", inputs); c != nil && c.String() != nil {
		negativePrompt = *c.String()
	}
	personGeneration := ""
	if c := core.FindConnection("person_generation", inputs); c != nil && c.String() != nil {
		personGeneration = *c.String()
	}
	var durationSeconds int64
	if c := core.FindConnection("duration_seconds", inputs); c != nil && c.Number() != nil {
		durationSeconds = *c.Number()
	}
	var seed int64
	if c := core.FindConnection("seed", inputs); c != nil && c.Number() != nil {
		seed = *c.Number()
	}

	// Optional input image — enables image-to-video mode. Accepts
	// either a flo:blob token (resolved via the blob store) or raw
	// base64 (used directly). Empty string skips image conditioning.
	var inputImageB64, inputImageMime string
	if c := core.FindConnection("input_image_blob", inputs); c != nil && c.String() != nil && strings.TrimSpace(*c.String()) != "" {
		raw := strings.TrimSpace(*c.String())
		if strings.HasPrefix(raw, "flo:blob:") {
			blobs := flow.Blobs()
			if blobs == nil {
				return errorResult("input_image_blob is a flo:blob token but no blob store is available", ""), nil
			}
			bytesData, err := blobs.Get(raw)
			if err != nil {
				return errorResult(fmt.Sprintf("failed to resolve input image blob: %v", err), ""), nil
			}
			// store.Get returns the raw bytes (or base64-string-form
			// bytes when the original Put took a base64 string). For
			// images we re-encode to base64 to feed Gemini — it's
			// cheap and avoids ambiguity about which form we have.
			inputImageB64 = base64.StdEncoding.EncodeToString(bytesData)
			inputImageMime = sniffImageMime(bytesData)
		} else {
			inputImageB64 = raw
			inputImageMime = "image/png"
		}
	}

	// Build the predictLongRunning request body. Shape pinned to the
	// v1beta API: instances[0].prompt (+ optional image), parameters
	// for everything else.
	instance := map[string]interface{}{"prompt": prompt}
	if inputImageB64 != "" {
		instance["image"] = map[string]interface{}{
			"bytesBase64Encoded": inputImageB64,
			"mimeType":           inputImageMime,
		}
	}
	parameters := map[string]interface{}{}
	if aspectRatio != "" {
		parameters["aspectRatio"] = aspectRatio
	}
	if durationSeconds > 0 {
		parameters["durationSeconds"] = durationSeconds
	}
	if negativePrompt != "" {
		parameters["negativePrompt"] = negativePrompt
	}
	if personGeneration != "" {
		parameters["personGeneration"] = personGeneration
	}
	if seed > 0 {
		parameters["seed"] = seed
	}
	// numberOfVideos / sampleCount intentionally omitted. Older Veo
	// versions accepted "numberOfVideos"; Veo 3.1 rejects unknown
	// parameters and treats sample count as implicitly 1. If a
	// future version supports multi-sample generation we'll re-add
	// the parameter conditional on the model identifier — sending
	// it unconditionally breaks the current preview models.

	payload := map[string]interface{}{
		"instances":  []map[string]interface{}{instance},
		"parameters": parameters,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to marshal request: %v", err), ""), nil
	}

	startURL := apiBase + "models/" + model + ":predictLongRunning"
	req, err := http.NewRequestWithContext(flow.GoContext(), http.MethodPost, startURL, bytes.NewReader(body))
	if err != nil {
		return errorResult(fmt.Sprintf("failed to create request: %v", err), ""), nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", apiKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return errorResult(fmt.Sprintf("Veo request failed: %v", err), ""), nil
	}
	startBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(startBody, &apiErr)
		msg := apiErr.Error.Message
		if msg == "" {
			msg = string(startBody)
		}
		return errorResult(fmt.Sprintf("Veo API error (%d): %s", resp.StatusCode, msg), ""), nil
	}

	var startResp struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(startBody, &startResp); err != nil {
		return errorResult(fmt.Sprintf("failed to parse Veo start response: %v", err), ""), nil
	}
	if startResp.Name == "" {
		return errorResult("Veo did not return an operation name", ""), nil
	}
	operationName := startResp.Name

	log.WithFields(log.Fields{
		"model":     model,
		"operation": operationName,
	}).Info("[gemini_video] LRO started; entering poll loop")

	// Poll loop. We use a manual ticker rather than time.Sleep so
	// the executor's GoContext cancellation (runner shutdown, user
	// cancel) tears the loop down immediately rather than waiting
	// out a 5-second sleep.
	deadline := time.Now().Add(pollTimeout)
	pollURL := apiBase + operationName
	pollClient := &http.Client{Timeout: 60 * time.Second}

	var doneBody []byte
pollLoop:
	for {
		if time.Now().After(deadline) {
			return errorResult(
				fmt.Sprintf("Veo generation did not complete within %s", pollTimeout),
				operationName,
			), nil
		}

		select {
		case <-flow.GoContext().Done():
			return errorResult("execution cancelled while polling Veo", operationName), nil
		case <-time.After(pollInterval):
		}

		pollReq, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, pollURL, nil)
		if err != nil {
			return errorResult(fmt.Sprintf("failed to build poll request: %v", err), operationName), nil
		}
		pollReq.Header.Set("x-goog-api-key", apiKey)

		pollResp, err := pollClient.Do(pollReq)
		if err != nil {
			// Transient network errors during polling — log and
			// retry on the next tick rather than killing the whole
			// generation. The deadline still bounds the overall
			// wait.
			log.WithError(err).Warn("[gemini_video] poll failed, retrying")
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(pollResp.Body, maxResponseBody))
		_ = pollResp.Body.Close()

		if pollResp.StatusCode != http.StatusOK {
			var apiErr struct {
				Error struct {
					Message string `json:"message"`
				} `json:"error"`
			}
			_ = json.Unmarshal(body, &apiErr)
			msg := apiErr.Error.Message
			if msg == "" {
				msg = string(body)
			}
			return errorResult(fmt.Sprintf("Veo poll error (%d): %s", pollResp.StatusCode, msg), operationName), nil
		}

		var statusResp struct {
			Done bool `json:"done"`
		}
		if err := json.Unmarshal(body, &statusResp); err != nil {
			return errorResult(fmt.Sprintf("failed to parse poll response: %v", err), operationName), nil
		}
		if statusResp.Done {
			doneBody = body
			break pollLoop
		}
	}

	// Parse the completed operation response. The shape Veo returns
	// has been observed in two variants — generatedSamples (Veo 2)
	// and generatedVideos (Veo 3 preview) — so we handle both.
	// Inside each sample/video we may get either a download URI or
	// inline base64 bytes.
	var done struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Response struct {
			GenerateVideoResponse struct {
				GeneratedSamples []struct {
					Video struct {
						URI                string `json:"uri,omitempty"`
						BytesBase64Encoded string `json:"bytesBase64Encoded,omitempty"`
						MimeType           string `json:"mimeType,omitempty"`
					} `json:"video"`
				} `json:"generatedSamples,omitempty"`
				GeneratedVideos []struct {
					Video struct {
						URI                string `json:"uri,omitempty"`
						BytesBase64Encoded string `json:"bytesBase64Encoded,omitempty"`
						MimeType           string `json:"mimeType,omitempty"`
					} `json:"video"`
				} `json:"generatedVideos,omitempty"`
			} `json:"generateVideoResponse"`
		} `json:"response"`
	}
	if err := json.Unmarshal(doneBody, &done); err != nil {
		return errorResult(fmt.Sprintf("failed to parse completed operation: %v", err), operationName), nil
	}
	if done.Error.Message != "" {
		return errorResult("Veo generation failed: "+done.Error.Message, operationName), nil
	}

	// Log the response shape (NOT the bytes) so we can diagnose
	// model-specific response variations. Veo 3.1 may carry the
	// video and audio in separate blocks compared to Veo 2, which
	// would explain audio-only playback if we picked the wrong
	// field. We log the top-level structure (which keys are present)
	// without ever surfacing base64 bytes — checkContainsKey returns
	// whether a key appears anywhere in the response JSON.
	log.WithFields(log.Fields{
		"model":             model,
		"operation":         operationName,
		"response_bytes":    len(doneBody),
		"samples_count":     len(done.Response.GenerateVideoResponse.GeneratedSamples),
		"videos_count":      len(done.Response.GenerateVideoResponse.GeneratedVideos),
		"has_video_key":     bytes.Contains(doneBody, []byte(`"video"`)),
		"has_audio_key":     bytes.Contains(doneBody, []byte(`"audio"`)),
		"has_uri_key":       bytes.Contains(doneBody, []byte(`"uri"`)),
		"has_bytes_key":     bytes.Contains(doneBody, []byte(`"bytesBase64Encoded"`)),
		"has_mime_key":      bytes.Contains(doneBody, []byte(`"mimeType"`)),
		"has_videos_key":    bytes.Contains(doneBody, []byte(`"generatedVideos"`)),
		"has_samples_key":   bytes.Contains(doneBody, []byte(`"generatedSamples"`)),
		"has_response_key":  bytes.Contains(doneBody, []byte(`"response"`)),
	}).Info("[gemini_video] LRO completed — response shape inspection")

	var videoURI, videoB64, videoMime string
	if samples := done.Response.GenerateVideoResponse.GeneratedSamples; len(samples) > 0 {
		videoURI = samples[0].Video.URI
		videoB64 = samples[0].Video.BytesBase64Encoded
		videoMime = samples[0].Video.MimeType
	}
	if videoURI == "" && videoB64 == "" {
		if videos := done.Response.GenerateVideoResponse.GeneratedVideos; len(videos) > 0 {
			videoURI = videos[0].Video.URI
			videoB64 = videos[0].Video.BytesBase64Encoded
			if videoMime == "" {
				videoMime = videos[0].Video.MimeType
			}
		}
	}
	if videoURI == "" && videoB64 == "" {
		return errorResult("Veo returned no video data", operationName), nil
	}

	var videoBytes []byte
	if videoB64 != "" {
		videoBytes, err = base64.StdEncoding.DecodeString(videoB64)
		if err != nil {
			return errorResult(fmt.Sprintf("failed to decode inline video bytes: %v", err), operationName), nil
		}
	} else {
		// Download from the signed URI. Gemini File API requires the
		// API key to be supplied as a query parameter on the URI.
		downloadURL := videoURI
		if !strings.Contains(downloadURL, "key=") {
			sep := "?"
			if strings.Contains(downloadURL, "?") {
				sep = "&"
			}
			downloadURL = downloadURL + sep + "key=" + apiKey
		}
		dlReq, err := http.NewRequestWithContext(flow.GoContext(), http.MethodGet, downloadURL, nil)
		if err != nil {
			return errorResult(fmt.Sprintf("failed to build video download request: %v", err), operationName), nil
		}
		dlClient := &http.Client{Timeout: 120 * time.Second}
		dlResp, err := dlClient.Do(dlReq)
		if err != nil {
			return errorResult(fmt.Sprintf("video download failed: %v", err), operationName), nil
		}
		videoBytes, _ = io.ReadAll(io.LimitReader(dlResp.Body, maxResponseBody))
		_ = dlResp.Body.Close()
		if dlResp.StatusCode != http.StatusOK {
			return errorResult(fmt.Sprintf("video download HTTP %d: %s", dlResp.StatusCode, truncate(string(videoBytes), 200)), operationName), nil
		}
		videoB64 = base64.StdEncoding.EncodeToString(videoBytes)
	}

	if videoMime == "" {
		videoMime = "video/mp4"
	}

	// Off-load to the blob store so downstream actions can consume
	// the video via flo:blob:<handle> instead of carrying the
	// multi-MB base64 through every node's output. Token resolution
	// is automatic on the way into any downstream action.
	var blobToken string
	if blobs := flow.Blobs(); blobs != nil {
		token, err := blobs.Put(videoBytes, videoMime)
		if err == nil {
			blobToken = token
		} else {
			log.WithError(err).Warn("[gemini_video] failed to off-load video to blob store; falling back to inline base64 only")
		}
	}

	toolResult := fmt.Sprintf("Generated video (%s, %d bytes)", videoMime, len(videoBytes))

	return map[string]interface{}{
		"tool_result":    toolResult,
		"video_blob":     blobToken,
		"video_base64":   videoB64,
		"mime_type":      videoMime,
		"model":          model,
		"operation_name": operationName,
		"success":        true,
		"error":          "",
	}, nil
}

// sniffImageMime returns a content-type for the supplied image bytes.
// Uses http.DetectContentType — Gemini accepts the common image
// formats (PNG, JPEG, WEBP). Falls back to image/png when sniffing
// can't pin a media type, which matches the behaviour of the existing
// gemini_image action's output default.
func sniffImageMime(b []byte) string {
	if len(b) == 0 {
		return "image/png"
	}
	ct := http.DetectContentType(b)
	if strings.HasPrefix(ct, "image/") {
		return ct
	}
	return "image/png"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// errorResult returns the canonical failure-shape output map. The
// optional operationName lets a caller (or a follow-up flow) re-fetch
// a still-running LRO if the action timed out before completion.
func errorResult(msg, operationName string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result":    "Video generation failed: " + msg,
		"video_blob":     "",
		"video_base64":   "",
		"mime_type":      "",
		"model":          "",
		"operation_name": operationName,
		"success":        false,
		"error":          msg,
	}
}
