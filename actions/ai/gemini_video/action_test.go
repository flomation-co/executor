// Gemini Video action tests. The action is asynchronous in shape —
// POST :predictLongRunning returns an operation handle, the action
// polls that operation until done, then surfaces the video bytes.
// These tests fake both halves of that exchange with a single
// httptest.Server and pin the AI-callable contract (tool_result on
// both success and failure paths, success=true/false reflecting
// reality, operation_name surfaced even on timeout for debugging).
//
// We override pollInterval to a few ms in tests so the suite stays
// fast; the production 5s value would make every test take a
// minimum of 5s per poll-completes-eventually case.
package gemini_video

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// stubServer wires up a httptest.Server and points apiBase at it for
// the duration of the test. Returns the server so handlers can be
// queried for hit counts / inspect bodies. Production polling
// intervals are dialled down to keep the test suite fast.
func stubServer(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	prevBase := apiBase
	prevPoll := pollInterval
	prevTimeout := pollTimeout
	apiBase = srv.URL + "/"
	pollInterval = 10 * time.Millisecond
	pollTimeout = 2 * time.Second
	t.Cleanup(func() {
		apiBase = prevBase
		pollInterval = prevPoll
		pollTimeout = prevTimeout
		srv.Close()
	})
	return srv
}

// TestExecute_MissingAPIKey — early-exit before any HTTP call.
func TestExecute_MissingAPIKey(t *testing.T) {
	RegisterTestingT(t)
	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "a sunset"},
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(result["success"]).To(BeFalse())
	Expect(result["error"]).To(ContainSubstring("api_key is required"))
	Expect(result["tool_result"]).To(ContainSubstring("api_key is required"))
}

// TestExecute_MissingPrompt — early-exit before any HTTP call.
func TestExecute_MissingPrompt(t *testing.T) {
	RegisterTestingT(t)
	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "AIza-test"},
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(result["success"]).To(BeFalse())
	Expect(result["error"]).To(ContainSubstring("prompt is required"))
}

// TestExecute_HappyPath_InlineBytes — model returns the video bytes
// inline via bytesBase64Encoded on the first poll. Validates the
// fastest path: no download hop, no multi-poll wait.
func TestExecute_HappyPath_InlineBytes(t *testing.T) {
	RegisterTestingT(t)

	fakeVideoBytes := []byte("MOCK-MP4-BYTES-FOR-TEST")
	fakeVideoB64 := base64.StdEncoding.EncodeToString(fakeVideoBytes)

	var startCalls, pollCalls int32
	stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, ":predictLongRunning"):
			atomic.AddInt32(&startCalls, 1)
			body, _ := io.ReadAll(r.Body)
			// Capture and verify the request shape — instances[0].prompt
			// must carry the user prompt.
			var req map[string]interface{}
			_ = json.Unmarshal(body, &req)
			instances := req["instances"].([]interface{})
			first := instances[0].(map[string]interface{})
			Expect(first["prompt"]).To(Equal("a starry night sky over the sea"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"operations/test-lro-1"}`))
		case strings.Contains(r.URL.Path, "operations/test-lro-1"):
			atomic.AddInt32(&pollCalls, 1)
			w.Header().Set("Content-Type", "application/json")
			payload := map[string]interface{}{
				"done": true,
				"response": map[string]interface{}{
					"generateVideoResponse": map[string]interface{}{
						"generatedSamples": []map[string]interface{}{
							{"video": map[string]interface{}{
								"bytesBase64Encoded": fakeVideoB64,
								"mimeType":           "video/mp4",
							}},
						},
					},
				},
			}
			b, _ := json.Marshal(payload)
			_, _ = w.Write(b)
		default:
			http.NotFound(w, r)
		}
	})

	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "AIza-test"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "a starry night sky over the sea"},
	})

	Expect(err).NotTo(HaveOccurred())
	Expect(result["success"]).To(BeTrue())
	Expect(result["error"]).To(Equal(""))
	Expect(result["mime_type"]).To(Equal("video/mp4"))
	Expect(result["video_base64"]).To(Equal(fakeVideoB64))
	Expect(result["operation_name"]).To(Equal("operations/test-lro-1"))
	Expect(result["tool_result"]).To(ContainSubstring("Generated video"))
	Expect(result["tool_result"]).To(ContainSubstring("video/mp4"))
	// Without a configured blob store, the token is empty but the
	// rest of the outputs still hold. This is the documented fallback.
	Expect(result["video_blob"]).To(Equal(""))
	Expect(atomic.LoadInt32(&startCalls)).To(Equal(int32(1)))
	Expect(atomic.LoadInt32(&pollCalls)).To(BeNumerically(">=", 1))
}

// TestExecute_PollCompletesEventually — the operation reports done=false
// for the first two polls before resolving. Pins the loop's correctness
// (it doesn't bail early on done=false) and the eventual happy-path
// completion.
func TestExecute_PollCompletesEventually(t *testing.T) {
	RegisterTestingT(t)

	fakeVideoBytes := []byte("EVENTUAL-MP4")
	fakeVideoB64 := base64.StdEncoding.EncodeToString(fakeVideoBytes)

	var pollCalls int32
	stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, ":predictLongRunning"):
			_, _ = w.Write([]byte(`{"name":"operations/slow-lro"}`))
		case strings.Contains(r.URL.Path, "operations/slow-lro"):
			n := atomic.AddInt32(&pollCalls, 1)
			if n < 3 {
				_, _ = w.Write([]byte(`{"done":false}`))
				return
			}
			payload := map[string]interface{}{
				"done": true,
				"response": map[string]interface{}{
					"generateVideoResponse": map[string]interface{}{
						"generatedVideos": []map[string]interface{}{
							{"video": map[string]interface{}{
								"bytesBase64Encoded": fakeVideoB64,
								"mimeType":           "video/mp4",
							}},
						},
					},
				},
			}
			b, _ := json.Marshal(payload)
			_, _ = w.Write(b)
		default:
			http.NotFound(w, r)
		}
	})

	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "AIza-test"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "anything"},
	})

	Expect(err).NotTo(HaveOccurred())
	Expect(result["success"]).To(BeTrue())
	Expect(result["video_base64"]).To(Equal(fakeVideoB64))
	Expect(atomic.LoadInt32(&pollCalls)).To(BeNumerically(">=", 3))
}

// TestExecute_Timeout — the operation never completes within the poll
// budget. The action must surface success=false with a clear timeout
// message AND retain the operation_name so a follow-up flow can in
// principle re-fetch the still-running LRO.
func TestExecute_Timeout(t *testing.T) {
	RegisterTestingT(t)

	stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, ":predictLongRunning"):
			_, _ = w.Write([]byte(`{"name":"operations/forever"}`))
		case strings.Contains(r.URL.Path, "operations/forever"):
			_, _ = w.Write([]byte(`{"done":false}`))
		}
	})
	// Pull the timeout down further than stubServer's default so this
	// test doesn't sit for the full 2s.
	prev := pollTimeout
	pollTimeout = 80 * time.Millisecond
	t.Cleanup(func() { pollTimeout = prev })

	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "AIza-test"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "anything"},
	})

	Expect(err).NotTo(HaveOccurred())
	Expect(result["success"]).To(BeFalse())
	Expect(result["error"]).To(ContainSubstring("did not complete"))
	Expect(result["tool_result"]).To(ContainSubstring("did not complete"))
	Expect(result["operation_name"]).To(Equal("operations/forever"))
}

// TestExecute_LROReportsError — the LRO resolves with done=true but
// carries an error block. Action surfaces the upstream message
// verbatim so the AI loop can react to safety filtering, quota
// rejection, etc.
func TestExecute_LROReportsError(t *testing.T) {
	RegisterTestingT(t)

	stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, ":predictLongRunning"):
			_, _ = w.Write([]byte(`{"name":"operations/will-fail"}`))
		case strings.Contains(r.URL.Path, "operations/will-fail"):
			_, _ = w.Write([]byte(`{"done":true,"error":{"message":"content policy violation"}}`))
		}
	})

	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "AIza-test"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "anything"},
	})

	Expect(err).NotTo(HaveOccurred())
	Expect(result["success"]).To(BeFalse())
	Expect(result["error"]).To(ContainSubstring("content policy violation"))
	Expect(result["operation_name"]).To(Equal("operations/will-fail"))
}

// TestExecute_StartRequestRejected — the predictLongRunning POST itself
// returns non-200. The action must surface the upstream message and
// NOT begin polling (otherwise we'd poll a non-existent operation
// forever).
func TestExecute_StartRequestRejected(t *testing.T) {
	RegisterTestingT(t)

	var pollCalls int32
	stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ":predictLongRunning") {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"invalid model"}}`))
			return
		}
		atomic.AddInt32(&pollCalls, 1)
		_, _ = w.Write([]byte(`{"done":true}`))
	})

	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "AIza-test"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "anything"},
	})

	Expect(err).NotTo(HaveOccurred())
	Expect(result["success"]).To(BeFalse())
	Expect(result["error"]).To(ContainSubstring("invalid model"))
	Expect(atomic.LoadInt32(&pollCalls)).To(Equal(int32(0)))
}

// TestExecute_DownloadURIFallback — the LRO resolves with a download
// URI instead of inline bytes. Action must download from the URI
// (with API key appended) and surface the resulting bytes. This is
// the path Veo 2 production responses actually take.
func TestExecute_DownloadURIFallback(t *testing.T) {
	RegisterTestingT(t)

	fakeVideoBytes := []byte("DOWNLOADED-MP4-BYTES")

	var downloadCalls int32
	stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, ":predictLongRunning"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"operations/uri-resp"}`))
		case strings.Contains(r.URL.Path, "operations/uri-resp"):
			// Build a download URI that points back at THIS server.
			uri := "http://" + r.Host + "/v1beta/files/test-clip:download"
			w.Header().Set("Content-Type", "application/json")
			payload := map[string]interface{}{
				"done": true,
				"response": map[string]interface{}{
					"generateVideoResponse": map[string]interface{}{
						"generatedSamples": []map[string]interface{}{
							{"video": map[string]interface{}{"uri": uri, "mimeType": "video/mp4"}},
						},
					},
				},
			}
			b, _ := json.Marshal(payload)
			_, _ = w.Write(b)
		case strings.HasSuffix(r.URL.Path, ":download"):
			atomic.AddInt32(&downloadCalls, 1)
			// Verify the API key was appended for download auth.
			Expect(r.URL.Query().Get("key")).To(Equal("AIza-test"))
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write(fakeVideoBytes)
		default:
			http.NotFound(w, r)
		}
	})

	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "AIza-test"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "anything"},
	})

	Expect(err).NotTo(HaveOccurred())
	Expect(result["success"]).To(BeTrue())
	Expect(result["mime_type"]).To(Equal("video/mp4"))
	decoded, _ := base64.StdEncoding.DecodeString(result["video_base64"].(string))
	Expect(decoded).To(Equal(fakeVideoBytes))
	Expect(atomic.LoadInt32(&downloadCalls)).To(Equal(int32(1)))
}

// TestExecute_ImageToVideo_PassesImageBytes — when input_image_blob is
// supplied as raw base64 (no flo:blob prefix), the request body must
// include instances[0].image.bytesBase64Encoded. The blob-store
// resolution path is not exercised here (requires a configured store);
// we cover the simpler "raw base64 passthrough" branch.
func TestExecute_ImageToVideo_PassesImageBytes(t *testing.T) {
	RegisterTestingT(t)

	rawImageBytes := []byte("FAKE-PNG-INPUT")
	rawImageB64 := base64.StdEncoding.EncodeToString(rawImageBytes)

	var capturedImage string
	stubServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, ":predictLongRunning"):
			body, _ := io.ReadAll(r.Body)
			var req map[string]interface{}
			_ = json.Unmarshal(body, &req)
			instances := req["instances"].([]interface{})
			first := instances[0].(map[string]interface{})
			if img, ok := first["image"].(map[string]interface{}); ok {
				capturedImage, _ = img["bytesBase64Encoded"].(string)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"operations/i2v"}`))
		case strings.Contains(r.URL.Path, "operations/i2v"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"done":true,"response":{"generateVideoResponse":{"generatedSamples":[{"video":{"bytesBase64Encoded":"AAA=","mimeType":"video/mp4"}}]}}}`))
		default:
			http.NotFound(w, r)
		}
	})

	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "AIza-test"},
		{Name: "prompt", Type: core.ConnectionTypeText, Value: "animate this still"},
		{Name: "input_image_blob", Type: core.ConnectionTypeText, Value: rawImageB64},
	})

	Expect(err).NotTo(HaveOccurred())
	Expect(result["success"]).To(BeTrue())
	Expect(capturedImage).To(Equal(rawImageB64))
}
