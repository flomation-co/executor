// Gemini Image action tests. These pin the AI-callable contract
// (tool_result populated on both success and failure paths, success
// boolean reflects what happened) and the response parsing of an
// inline_data part into the action's outputs.
//
// The blob store path is intentionally NOT exercised here — that
// requires a configured BlobStore (apiURL, ownerID, executionID).
// Tests assert the base64 + mime are surfaced regardless of blob
// availability so downstream actions can fall back if needed.
package gemini_image

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func stubServer(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	prev := apiBase
	apiBase = srv.URL + "/v1beta/models/"
	t.Cleanup(func() {
		apiBase = prev
		srv.Close()
	})
	return srv
}

// TestExecute_MissingAPIKey verifies that early-exit paths return an
// errorResult (success=false, tool_result populated) rather than a Go
// error. AI-callable actions need a structured failure shape so the
// agent loop can surface "tool failed because..." to the user.
func TestExecute_MissingAPIKey(t *testing.T) {
	RegisterTestingT(t)
	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "a cat"},
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(result["success"]).To(BeFalse())
	Expect(result["error"]).To(ContainSubstring("api_key is required"))
	Expect(result["tool_result"]).To(ContainSubstring("api_key is required"))
}

func TestExecute_MissingPrompt(t *testing.T) {
	RegisterTestingT(t)
	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "AIza-test"},
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(result["success"]).To(BeFalse())
	Expect(result["error"]).To(ContainSubstring("prompt is required"))
}

// TestExecute_HappyPath_DecodesImage validates the inline_data → base64
// pipeline AND that we asked Gemini for image modality (without
// response_modalities the model defaults to text-only).
func TestExecute_HappyPath_DecodesImage(t *testing.T) {
	RegisterTestingT(t)

	// 4-byte PNG-ish payload — enough to verify the decode round-trip.
	rawBytes := []byte{0x89, 'P', 'N', 'G'}
	encoded := base64.StdEncoding.EncodeToString(rawBytes)

	var capturedBody map[string]interface{}
	stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]interface{}{
							{
								"inlineData": map[string]interface{}{
									"mimeType": "image/png",
									"data":     encoded,
								},
							},
						},
					},
				},
			},
			"modelVersion": "gemini-2.5-flash-image-001",
		})
	})

	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "AIza-test"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "A photorealistic cat"},
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(result["success"]).To(BeTrue())
	Expect(result["mime_type"]).To(Equal("image/png"))
	Expect(result["image_base64"]).To(Equal(encoded))
	Expect(result["model"]).To(Equal("gemini-2.5-flash-image-001"))
	Expect(result["tool_result"]).To(ContainSubstring("Generated image"))
	Expect(result["tool_result"]).To(ContainSubstring("image/png"))

	// Confirm we asked for image modality so a text-only model
	// configuration drift gets caught here.
	genCfg, _ := capturedBody["generationConfig"].(map[string]interface{})
	mods, _ := genCfg["responseModalities"].([]interface{})
	Expect(mods).To(ContainElement("IMAGE"))
}

// TestExecute_NoImagePartSurfacesText guards the safety-refusal case.
// When Gemini blocks the prompt it returns a candidate with only a
// text part explaining why. The action must surface that reason
// instead of silently saying "no image returned".
func TestExecute_NoImagePartSurfacesText(t *testing.T) {
	RegisterTestingT(t)

	stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]interface{}{
							{"text": "I cannot generate that image because of policy"},
						},
					},
				},
			},
		})
	})

	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "AIza-test"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "something blocked"},
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(result["success"]).To(BeFalse())
	Expect(result["error"]).To(ContainSubstring("policy"))
	Expect(result["tool_result"]).To(ContainSubstring("policy"))
}

// TestExecute_APIError surfaces upstream HTTP errors into the
// errorResult shape (not a Go-level error) so an AI agent calling
// this as a tool can keep going.
func TestExecute_APIError(t *testing.T) {
	RegisterTestingT(t)

	stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message": "PermissionDenied: imagen access disabled",
			},
		})
	})

	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "AIza-test"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "a cat"},
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(result["success"]).To(BeFalse())
	Expect(result["error"]).To(ContainSubstring("403"))
	Expect(result["error"]).To(ContainSubstring("imagen access disabled"))
}
