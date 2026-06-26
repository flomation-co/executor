// Gemini TTS action tests. Mirror the image action's shape — pin
// validation, the request asks for AUDIO modality with the named
// voice, and the inline_data audio response decodes correctly with
// the mime preserved (Gemini returns "audio/L16;codec=pcm;rate=24000"
// which downstream players need intact to interpret the raw PCM).
package gemini_tts

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

func TestExecute_MissingAPIKey(t *testing.T) {
	RegisterTestingT(t)
	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "hi"},
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(result["success"]).To(BeFalse())
	Expect(result["error"]).To(ContainSubstring("api_key is required"))
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

// TestExecute_HappyPath_DecodesAudio confirms the request asks for
// AUDIO modality with the chosen voice and parses the inline_data
// audio back out. The mime is preserved verbatim — important because
// Gemini TTS returns parametrised mimes like
// "audio/L16;codec=pcm;rate=24000" that downstream players need.
func TestExecute_HappyPath_DecodesAudio(t *testing.T) {
	RegisterTestingT(t)

	rawBytes := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	encoded := base64.StdEncoding.EncodeToString(rawBytes)
	pcmMime := "audio/L16;codec=pcm;rate=24000"

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
									"mimeType": pcmMime,
									"data":     encoded,
								},
							},
						},
					},
				},
			},
			"modelVersion": "gemini-2.5-flash-preview-tts-001",
		})
	})

	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "AIza-test"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "Say cheerfully: hello, world!"},
		{Name: "voice", Type: core.ConnectionTypeSecret, Value: "Puck"},
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(result["success"]).To(BeTrue())
	Expect(result["mime_type"]).To(Equal(pcmMime))
	Expect(result["audio_base64"]).To(Equal(encoded))
	Expect(result["voice"]).To(Equal("Puck"))
	Expect(result["model"]).To(Equal("gemini-2.5-flash-preview-tts-001"))
	Expect(result["tool_result"]).To(ContainSubstring("Generated audio"))
	Expect(result["tool_result"]).To(ContainSubstring("Puck"))

	// Confirm we asked for AUDIO modality with the chosen voice.
	genCfg, _ := capturedBody["generationConfig"].(map[string]interface{})
	mods, _ := genCfg["responseModalities"].([]interface{})
	Expect(mods).To(ContainElement("AUDIO"))
	speech, _ := genCfg["speechConfig"].(map[string]interface{})
	voiceCfg, _ := speech["voiceConfig"].(map[string]interface{})
	prebuilt, _ := voiceCfg["prebuiltVoiceConfig"].(map[string]interface{})
	Expect(prebuilt["voiceName"]).To(Equal("Puck"))
}

// TestExecute_DefaultVoice catches a regression where the action
// stopped sending a voice config when the user left the input blank.
// The Kore default keeps the flow working without an explicit choice.
func TestExecute_DefaultVoice(t *testing.T) {
	RegisterTestingT(t)

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
									"mimeType": "audio/L16;codec=pcm;rate=24000",
									"data":     base64.StdEncoding.EncodeToString([]byte{0x00}),
								},
							},
						},
					},
				},
			},
		})
	})

	result, _ := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "AIza-test"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "hi"},
	})
	Expect(result["voice"]).To(Equal("Kore"))

	genCfg, _ := capturedBody["generationConfig"].(map[string]interface{})
	speech, _ := genCfg["speechConfig"].(map[string]interface{})
	voiceCfg, _ := speech["voiceConfig"].(map[string]interface{})
	prebuilt, _ := voiceCfg["prebuiltVoiceConfig"].(map[string]interface{})
	Expect(prebuilt["voiceName"]).To(Equal("Kore"))
}

// TestExecute_NoAudioPartFails — Gemini occasionally returns a
// candidate with no inline_data (model-side error / safety). The
// action must surface that as an errorResult, not silently produce
// success=true with empty audio.
func TestExecute_NoAudioPartFails(t *testing.T) {
	RegisterTestingT(t)

	stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"candidates": []map[string]interface{}{
				{
					"content": map[string]interface{}{
						"parts": []map[string]interface{}{},
					},
				},
			},
		})
	})

	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "AIza-test"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "hi"},
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(result["success"]).To(BeFalse())
	Expect(result["error"]).To(ContainSubstring("did not return audio"))
}
