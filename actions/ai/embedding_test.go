package ai_common

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	. "github.com/onsi/gomega"
)

// Every test here runs against an httptest server — no network, no credentials,
// no env. The providers' quirks are the point: each of these asserts a specific
// thing that would silently corrupt a vector store if it were wrong.

// ---------------------------------------------------------------------------
// Test doubles
// ---------------------------------------------------------------------------

// capture records what a provider actually received.
type capture struct {
	mu       sync.Mutex
	paths    []string
	bodies   []map[string]any
	auths    []string
	requests int
}

func (c *capture) record(r *http.Request) map[string]any {
	raw, _ := io.ReadAll(r.Body)
	var body map[string]any
	_ = json.Unmarshal(raw, &body)

	c.mu.Lock()
	defer c.mu.Unlock()
	c.paths = append(c.paths, r.URL.Path)
	c.bodies = append(c.bodies, body)
	c.auths = append(c.auths, r.Header.Get("Authorization"))
	c.requests++
	return body
}

func (c *capture) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.requests
}

// openAIData is one entry of an OpenAI embeddings response.
type openAIData struct {
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}

func openAIServer(t *testing.T, rec *capture, reply func(body map[string]any) (int, any)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := rec.record(r)
		status, payload := reply(body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(payload)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// inputsOf pulls the "input" array out of a captured request body.
func inputsOf(body map[string]any) []string {
	raw, _ := body["input"].([]any)
	out := make([]string, len(raw))
	for i, v := range raw {
		out[i], _ = v.(string)
	}
	return out
}

// ---------------------------------------------------------------------------
// OpenAI / OpenAI-compatible
// ---------------------------------------------------------------------------

func TestEmbed_OpenAI_RequestShape(t *testing.T) {
	RegisterTestingT(t)

	rec := &capture{}
	srv := openAIServer(t, rec, func(body map[string]any) (int, any) {
		data := []openAIData{}
		for i := range inputsOf(body) {
			data = append(data, openAIData{Index: i, Embedding: []float32{float32(i), 0.5}})
		}
		return 200, map[string]any{"data": data}
	})

	cfg := EmbedConfig{Provider: "openai_compatible", BaseURL: srv.URL, APIKey: "sk-test", Model: "text-embedding-3-small"}
	got, err := Embed(context.Background(), cfg, []string{"alpha", "beta"})
	Expect(err).ToNot(HaveOccurred())
	Expect(got).To(Equal([][]float32{{0, 0.5}, {1, 0.5}}))

	Expect(rec.count()).To(Equal(1))
	Expect(rec.paths[0]).To(Equal("/v1/embeddings"))
	Expect(rec.auths[0]).To(Equal("Bearer sk-test"))
	Expect(rec.bodies[0]["model"]).To(Equal("text-embedding-3-small"))
	Expect(inputsOf(rec.bodies[0])).To(Equal([]string{"alpha", "beta"}))
}

// `dimensions` is only honoured by text-embedding-3-*; ada-002 rejects the
// request outright if it is present. So the code gates on the model prefix, and
// getting that gate wrong is a hard failure for every ada-002 user.
func TestEmbed_OpenAI_DimensionsOnlyForTextEmbedding3(t *testing.T) {
	RegisterTestingT(t)

	tests := []struct {
		name       string
		model      string
		dimensions int
		wantSent   bool
	}{
		{"3-small with dimensions", "text-embedding-3-small", 512, true},
		{"3-large with dimensions", "text-embedding-3-large", 1024, true},
		{"ada-002 must NOT be sent dimensions", "text-embedding-ada-002", 512, false},
		{"unknown model must NOT be sent dimensions", "some-local-model", 512, false},
		{"3-small with no dimensions set", "text-embedding-3-small", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RegisterTestingT(t)

			rec := &capture{}
			srv := openAIServer(t, rec, func(body map[string]any) (int, any) {
				return 200, map[string]any{"data": []openAIData{{Index: 0, Embedding: []float32{0.1}}}}
			})

			cfg := EmbedConfig{
				Provider: "openai_compatible", BaseURL: srv.URL, APIKey: "sk-test",
				Model: tt.model, Dimensions: tt.dimensions,
			}
			_, err := Embed(context.Background(), cfg, []string{"x"})
			Expect(err).ToNot(HaveOccurred())

			_, sent := rec.bodies[0]["dimensions"]
			Expect(sent).To(Equal(tt.wantSent),
				"model %q dimensions=%d: `dimensions` in request body = %v, want %v",
				tt.model, tt.dimensions, sent, tt.wantSent)
			if tt.wantSent {
				Expect(rec.bodies[0]["dimensions"]).To(Equal(float64(tt.dimensions)))
			}
		})
	}
}

// The real correctness trap. The API documents that `data` comes back in input
// order, but it also carries an explicit `index` — and the vectors MUST be
// re-ordered by it. If they are not, a batch insert silently pairs every
// document with somebody else's embedding, and nothing ever errors: the search
// just returns nonsense forever.
func TestEmbed_OpenAI_ReordersByIndexNotArrivalOrder(t *testing.T) {
	RegisterTestingT(t)

	rec := &capture{}
	srv := openAIServer(t, rec, func(body map[string]any) (int, any) {
		// Deliberately shuffled: 2, 0, 3, 1.
		return 200, map[string]any{"data": []openAIData{
			{Index: 2, Embedding: []float32{2, 2}},
			{Index: 0, Embedding: []float32{0, 0}},
			{Index: 3, Embedding: []float32{3, 3}},
			{Index: 1, Embedding: []float32{1, 1}},
		}}
	})

	cfg := EmbedConfig{Provider: "openai_compatible", BaseURL: srv.URL, Model: "text-embedding-3-small"}
	got, err := Embed(context.Background(), cfg, []string{"zero", "one", "two", "three"})
	Expect(err).ToNot(HaveOccurred())

	// Back in INPUT order, not arrival order.
	Expect(got).To(Equal([][]float32{{0, 0}, {1, 1}, {2, 2}, {3, 3}}),
		"embeddings must be re-ordered by the response's `index` field, not taken in arrival order")
}

// An index we never sent means the reply does not correspond to the request —
// better to fail than to write a vector against the wrong document.
func TestEmbed_OpenAI_OutOfRangeIndex(t *testing.T) {
	RegisterTestingT(t)

	rec := &capture{}
	srv := openAIServer(t, rec, func(body map[string]any) (int, any) {
		return 200, map[string]any{"data": []openAIData{
			{Index: 0, Embedding: []float32{0.1}},
			{Index: 7, Embedding: []float32{0.2}}, // never sent
		}}
	})

	cfg := EmbedConfig{Provider: "openai_compatible", BaseURL: srv.URL, Model: "text-embedding-3-small"}
	_, err := Embed(context.Background(), cfg, []string{"a", "b"})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("wasn't sent"))
}

// OpenAI caps the inputs per request, and a self-hosted compatible server
// usually caps it lower still — so a batch bigger than the limit has to be
// split, and reassembled in order.
func TestEmbed_OpenAI_SplitsBatchesOver96(t *testing.T) {
	RegisterTestingT(t)

	rec := &capture{}
	srv := openAIServer(t, rec, func(body map[string]any) (int, any) {
		// (No assertions in here — this runs on the server's goroutine. The
		// chunk sizes are checked from the test goroutine below.)
		inputs := inputsOf(body)
		data := make([]openAIData, len(inputs))
		for i, text := range inputs {
			// Echo the text back as the vector so we can prove the reassembly
			// puts every embedding against the right document.
			var n float32
			_, _ = fmt.Sscanf(text, "doc-%f", &n)
			data[i] = openAIData{Index: i, Embedding: []float32{n}}
		}
		return 200, map[string]any{"data": data}
	})

	texts := make([]string, 250)
	for i := range texts {
		texts[i] = fmt.Sprintf("doc-%d", i)
	}

	cfg := EmbedConfig{Provider: "openai_compatible", BaseURL: srv.URL, Model: "text-embedding-3-small"}
	got, err := Embed(context.Background(), cfg, texts)
	Expect(err).ToNot(HaveOccurred())

	// 250 inputs at 96 per request = 96 + 96 + 58.
	Expect(rec.count()).To(Equal(3))
	Expect(inputsOf(rec.bodies[0])).To(HaveLen(96))
	Expect(inputsOf(rec.bodies[1])).To(HaveLen(96))
	Expect(inputsOf(rec.bodies[2])).To(HaveLen(58))

	Expect(got).To(HaveLen(250))
	for i := range got {
		Expect(got[i]).To(Equal([]float32{float32(i)}),
			"document %d got the embedding for document %v after batch reassembly", i, got[i])
	}
}

func TestEmbed_OpenAI_EmptyEmbedding(t *testing.T) {
	RegisterTestingT(t)

	rec := &capture{}
	srv := openAIServer(t, rec, func(body map[string]any) (int, any) {
		return 200, map[string]any{"data": []openAIData{
			{Index: 0, Embedding: []float32{0.1}},
			{Index: 1, Embedding: []float32{}}, // empty
		}}
	})

	cfg := EmbedConfig{Provider: "openai_compatible", BaseURL: srv.URL, Model: "text-embedding-3-small"}
	_, err := Embed(context.Background(), cfg, []string{"a", "b"})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("empty embedding for document 2"))
}

func TestEmbed_OpenAI_CountMismatch(t *testing.T) {
	RegisterTestingT(t)

	rec := &capture{}
	srv := openAIServer(t, rec, func(body map[string]any) (int, any) {
		return 200, map[string]any{"data": []openAIData{{Index: 0, Embedding: []float32{0.1}}}}
	})

	cfg := EmbedConfig{Provider: "openai_compatible", BaseURL: srv.URL, Model: "text-embedding-3-small"}
	_, err := Embed(context.Background(), cfg, []string{"a", "b"})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("asked the model for 2 embeddings but got 1 back"))
}

// A 200 carrying an error object (some compatible servers do this) is still an
// error.
func TestEmbed_OpenAI_ErrorInA200Body(t *testing.T) {
	RegisterTestingT(t)

	rec := &capture{}
	srv := openAIServer(t, rec, func(body map[string]any) (int, any) {
		return 200, map[string]any{"error": map[string]any{"message": "model not found"}}
	})

	cfg := EmbedConfig{Provider: "openai_compatible", BaseURL: srv.URL, Model: "nope"}
	_, err := Embed(context.Background(), cfg, []string{"a"})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(Equal("model not found"))
}

// ---------------------------------------------------------------------------
// Azure OpenAI
// ---------------------------------------------------------------------------

// Azure speaks the OpenAI request/response shape but re-plumbs everything
// around it: the model is a customer-named DEPLOYMENT in the URL path, every
// call carries an api-version query param, and the key travels in the
// non-standard api-key header — Authorization: Bearer is rejected for keys.
func TestEmbed_AzureOpenAI_RequestShape(t *testing.T) {
	RegisterTestingT(t)

	var gotPath, gotQuery, gotAPIKey, gotAuth string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotAPIKey = r.Header.Get("api-key")
		gotAuth = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []openAIData{
			{Index: 0, Embedding: []float32{0.1, 0.2}},
			{Index: 1, Embedding: []float32{0.3, 0.4}},
		}})
	}))
	t.Cleanup(srv.Close)

	cfg := EmbedConfig{Provider: "azure_openai", BaseURL: srv.URL, APIKey: "azure-key", Model: "my-embed-deployment"}
	got, err := Embed(context.Background(), cfg, []string{"alpha", "beta"})
	Expect(err).ToNot(HaveOccurred())
	Expect(got).To(Equal([][]float32{{0.1, 0.2}, {0.3, 0.4}}))

	Expect(gotPath).To(Equal("/openai/deployments/my-embed-deployment/embeddings"))
	Expect(gotQuery).To(Equal("api-version=2024-10-21"), "an empty APIVersion must default, not send an empty param")
	Expect(gotAPIKey).To(Equal("azure-key"))
	Expect(gotAuth).To(Equal(""), "Azure key auth must not send Authorization: Bearer")
	Expect(inputsOf(gotBody)).To(Equal([]string{"alpha", "beta"}))
}

func TestEmbed_AzureOpenAI_APIVersionOverride(t *testing.T) {
	RegisterTestingT(t)

	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []openAIData{{Index: 0, Embedding: []float32{0.1}}}})
	}))
	t.Cleanup(srv.Close)

	cfg := EmbedConfig{Provider: "azure_openai", BaseURL: srv.URL, APIKey: "azure-key", Model: "embed", APIVersion: "2025-03-01-preview"}
	_, err := Embed(context.Background(), cfg, []string{"a"})
	Expect(err).ToNot(HaveOccurred())
	Expect(gotQuery).To(Equal("api-version=2025-03-01-preview"))
}

// A deployment name says nothing about the model behind it, so Azure cannot
// reuse the text-embedding-3 prefix gate — dimensions pass through whenever
// set, and stay out of the body when not.
func TestEmbed_AzureOpenAI_DimensionsPassThrough(t *testing.T) {
	RegisterTestingT(t)

	tests := []struct {
		name       string
		dimensions int
		wantSent   bool
	}{
		{"dimensions set on an arbitrary deployment name", 1024, true},
		{"dimensions unset", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RegisterTestingT(t)

			var gotBody map[string]any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(raw, &gotBody)
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"data": []openAIData{{Index: 0, Embedding: []float32{0.1}}}})
			}))
			t.Cleanup(srv.Close)

			cfg := EmbedConfig{
				Provider: "azure_openai", BaseURL: srv.URL, APIKey: "azure-key",
				Model: "prod-embedder", Dimensions: tt.dimensions,
			}
			_, err := Embed(context.Background(), cfg, []string{"x"})
			Expect(err).ToNot(HaveOccurred())

			_, sent := gotBody["dimensions"]
			Expect(sent).To(Equal(tt.wantSent))
			if tt.wantSent {
				Expect(gotBody["dimensions"]).To(Equal(float64(tt.dimensions)))
			}
		})
	}
}

// Azure shares OpenAI's per-request input cap, so the same batching applies.
func TestEmbed_AzureOpenAI_SplitsBatches(t *testing.T) {
	RegisterTestingT(t)

	rec := &capture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := rec.record(r)
		inputs := inputsOf(body)
		data := make([]openAIData, len(inputs))
		for i, text := range inputs {
			var n float32
			_, _ = fmt.Sscanf(text, "doc-%f", &n)
			data[i] = openAIData{Index: i, Embedding: []float32{n}}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	t.Cleanup(srv.Close)

	texts := make([]string, 100)
	for i := range texts {
		texts[i] = fmt.Sprintf("doc-%d", i)
	}

	cfg := EmbedConfig{Provider: "azure_openai", BaseURL: srv.URL, APIKey: "azure-key", Model: "embed"}
	got, err := Embed(context.Background(), cfg, texts)
	Expect(err).ToNot(HaveOccurred())

	// 100 inputs at 96 per request = 96 + 4.
	Expect(rec.count()).To(Equal(2))
	Expect(inputsOf(rec.bodies[0])).To(HaveLen(96))
	Expect(inputsOf(rec.bodies[1])).To(HaveLen(4))

	Expect(got).To(HaveLen(100))
	for i := range got {
		Expect(got[i]).To(Equal([]float32{float32(i)}),
			"document %d got the embedding for document %v after batch reassembly", i, got[i])
	}
}

func TestEmbed_AzureOpenAI_Preconditions(t *testing.T) {
	RegisterTestingT(t)

	// No endpoint to build the deployment URL from — must fail before any dial.
	_, err := Embed(context.Background(), EmbedConfig{Provider: "azure_openai", APIKey: "k"}, []string{"a"})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("needs the resource endpoint"))

	// Azure's data plane always wants a key.
	_, err = Embed(context.Background(), EmbedConfig{Provider: "azure_openai", BaseURL: "https://r.openai.azure.com"}, []string{"a"})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(Equal("Azure OpenAI needs an API Key"))
}

// ---------------------------------------------------------------------------
// Ollama
// ---------------------------------------------------------------------------

// /api/embed is the BATCH endpoint. The legacy /api/embeddings takes one input
// and spells its response key differently ("embedding", not "embeddings"), so
// pointing at the wrong one silently returns nothing usable.
func TestEmbed_Ollama_UsesTheBatchEndpoint(t *testing.T) {
	RegisterTestingT(t)

	rec := &capture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.record(r)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"embeddings": [][]float32{{0.1, 0.2}, {0.3, 0.4}},
		})
	}))
	t.Cleanup(srv.Close)

	cfg := EmbedConfig{Provider: "ollama", BaseURL: srv.URL, Model: "nomic-embed-text"}
	got, err := Embed(context.Background(), cfg, []string{"alpha", "beta"})
	Expect(err).ToNot(HaveOccurred())
	Expect(got).To(Equal([][]float32{{0.1, 0.2}, {0.3, 0.4}}))

	Expect(rec.paths[0]).To(Equal("/api/embed"),
		"the legacy /api/embeddings is single-input and returns a differently-named key")
	Expect(rec.paths[0]).ToNot(Equal("/api/embeddings"))
	Expect(rec.bodies[0]["model"]).To(Equal("nomic-embed-text"))
	Expect(inputsOf(rec.bodies[0])).To(Equal([]string{"alpha", "beta"}))

	// Ollama has no auth, and there is no API Key input for it.
	Expect(rec.auths[0]).To(Equal(""))

	// Ollama sends the whole batch in one request — no 96-input cap.
	Expect(rec.count()).To(Equal(1))
}

// A chat model pointed at /api/embed returns nothing useful. Saying "is that an
// embedding model?" is the difference between a five-minute fix and an hour of
// staring at an empty result.
func TestEmbed_Ollama_CountMismatchAsksTheRightQuestion(t *testing.T) {
	RegisterTestingT(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"embeddings": [][]float32{}})
	}))
	t.Cleanup(srv.Close)

	cfg := EmbedConfig{Provider: "ollama", BaseURL: srv.URL, Model: "llama3"}
	_, err := Embed(context.Background(), cfg, []string{"a", "b"})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("asked Ollama for 2 embeddings but got 0 back"))
	Expect(err.Error()).To(ContainSubstring(`is "llama3" an embedding model?`))
}

func TestEmbed_Ollama_ErrorField(t *testing.T) {
	RegisterTestingT(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"error": `model "llama3" not found, try pulling it first`})
	}))
	t.Cleanup(srv.Close)

	cfg := EmbedConfig{Provider: "ollama", BaseURL: srv.URL, Model: "llama3"}
	_, err := Embed(context.Background(), cfg, []string{"a"})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("try pulling it first"))
}

func TestEmbed_Ollama_TrailingSlashInBaseURL(t *testing.T) {
	RegisterTestingT(t)

	rec := &capture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.record(r)
		_ = json.NewEncoder(w).Encode(map[string]any{"embeddings": [][]float32{{0.1}}})
	}))
	t.Cleanup(srv.Close)

	cfg := EmbedConfig{Provider: "ollama", BaseURL: srv.URL + "/", Model: "nomic-embed-text"}
	_, err := Embed(context.Background(), cfg, []string{"a"})
	Expect(err).ToNot(HaveOccurred())
	Expect(rec.paths[0]).To(Equal("/api/embed"), "a trailing slash must not produce //api/embed")
}

// ---------------------------------------------------------------------------
// Error surfacing and redaction
// ---------------------------------------------------------------------------

// Two things at once. The provider's OWN message must reach the operator —
// "Incorrect API key provided" is actionable, "HTTP 401" is not. And the API key
// must be scrubbed out of it: a provider that echoes the key back in its error
// text (OpenAI does, partially masked; a compatible server may not mask at all)
// would otherwise write the operator's secret into the flow's error output,
// which is stored and shown in the UI.
func TestEmbed_ErrorSurfacesProviderMessageAndRedactsTheKey(t *testing.T) {
	RegisterTestingT(t)

	const apiKey = "sk-supersecret-do-not-leak"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				// The provider echoes the key straight back at us.
				"message": "Incorrect API key provided: " + apiKey + ". You can find your API key at https://platform.openai.com/account/api-keys.",
				"type":    "invalid_request_error",
				"code":    "invalid_api_key",
			},
		})
	}))
	t.Cleanup(srv.Close)

	cfg := EmbedConfig{Provider: "openai_compatible", BaseURL: srv.URL, APIKey: apiKey, Model: "text-embedding-3-small"}
	_, err := Embed(context.Background(), cfg, []string{"a"})
	Expect(err).To(HaveOccurred())

	msg := err.Error()

	// The provider's own words, not "HTTP 401".
	Expect(msg).To(ContainSubstring("Incorrect API key provided"))
	Expect(msg).To(ContainSubstring("401")) // the status is still useful context

	// And the key is gone.
	Expect(msg).ToNot(ContainSubstring(apiKey), "the API key leaked into the error message: %s", msg)
	Expect(msg).To(ContainSubstring("********"))
}

// A non-JSON error body still has to say something useful.
func TestEmbed_ErrorFromNonJSONBody(t *testing.T) {
	RegisterTestingT(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("502 Bad Gateway (nginx)"))
	}))
	t.Cleanup(srv.Close)

	cfg := EmbedConfig{Provider: "openai_compatible", BaseURL: srv.URL, Model: "text-embedding-3-small"}
	_, err := Embed(context.Background(), cfg, []string{"a"})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("502"))
	Expect(err.Error()).To(ContainSubstring("nginx"))
}

// A flat {"message": ...} body (what several compatible servers emit) is picked
// up too.
func TestEmbed_ErrorFromFlatMessageBody(t *testing.T) {
	RegisterTestingT(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"message": "no such deployment"})
	}))
	t.Cleanup(srv.Close)

	cfg := EmbedConfig{Provider: "openai_compatible", BaseURL: srv.URL, Model: "text-embedding-3-small"}
	_, err := Embed(context.Background(), cfg, []string{"a"})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("no such deployment"))
}

func TestRedactEmbed(t *testing.T) {
	RegisterTestingT(t)

	cfg := EmbedConfig{APIKey: "sk-abc", SecretKey: "aws-secret"}
	got := RedactEmbed(cfg, "auth failed for sk-abc using aws-secret (sk-abc again)")
	Expect(got).ToNot(ContainSubstring("sk-abc"))
	Expect(got).ToNot(ContainSubstring("aws-secret"))

	// Empty credentials must not turn every empty string into asterisks.
	Expect(RedactEmbed(EmbedConfig{}, "nothing secret")).To(Equal("nothing secret"))
}

// ---------------------------------------------------------------------------
// Input validation — before we ever leave the process
// ---------------------------------------------------------------------------

func TestEmbed_RejectsEmptyText(t *testing.T) {
	RegisterTestingT(t)

	// A server that must never be reached.
	reached := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
	}))
	t.Cleanup(srv.Close)

	cfg := EmbedConfig{Provider: "openai_compatible", BaseURL: srv.URL, Model: "text-embedding-3-small"}

	tests := []struct {
		name    string
		texts   []string
		wantErr string
	}{
		{"no texts at all", nil, "there is no text to embed"},
		{"empty slice", []string{}, "there is no text to embed"},
		{"empty string", []string{""}, "document 1 has no text to embed"},
		{"whitespace only", []string{"   \t\n  "}, "document 1 has no text to embed"},
		{"blank in the middle of a batch", []string{"a", "  ", "c"}, "document 2 has no text to embed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RegisterTestingT(t)
			_, err := Embed(context.Background(), cfg, tt.texts)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(tt.wantErr))
		})
	}

	Expect(reached).To(BeFalse(), "an empty text must be rejected before any request is made")
}

func TestEmbed_ProviderPreconditions(t *testing.T) {
	RegisterTestingT(t)

	// OpenAI proper always needs a key — and this must fail before any dial, so
	// the test is safe with no network.
	_, err := Embed(context.Background(), EmbedConfig{Provider: "openai"}, []string{"a"})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(Equal("OpenAI needs an API Key"))

	// An OpenAI-compatible server has no default host to fall back on.
	_, err = Embed(context.Background(), EmbedConfig{Provider: "openai_compatible"}, []string{"a"})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("needs a Base URL"))

	// An unknown provider.
	_, err = Embed(context.Background(), EmbedConfig{Provider: "cohere"}, []string{"a"})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring(`"cohere" isn't an embedding provider we support`))
}

// Every provider the dropdown offers must be one Embed actually dispatches on —
// an option that falls through to "isn't an embedding provider we support" is a
// dead entry in the UI. (Checked structurally rather than by calling Embed:
// bedrock and ollama would dial out, and these tests do no network.)
func TestEmbed_EveryDropdownProviderIsDispatched(t *testing.T) {
	RegisterTestingT(t)

	dispatched := map[string]bool{
		"":                  true, // an unset provider defaults to OpenAI
		"openai":            true,
		"openai_compatible": true,
		"azure_openai":      true,
		"ollama":            true,
		"bedrock":           true,
	}

	Expect(EmbedProviderOptions).ToNot(BeEmpty())
	for _, opt := range EmbedProviderOptions {
		Expect(dispatched).To(HaveKey(opt.Value),
			"provider %q is offered in the dropdown but Embed's switch doesn't handle it", opt.Value)
		Expect(opt.Name).ToNot(BeEmpty())
	}

	// OpenRouter is deliberately absent: it proxies chat completions only and
	// has no embeddings endpoint.
	for _, opt := range EmbedProviderOptions {
		Expect(opt.Value).ToNot(Equal("openrouter"))
	}
}

// An unset model falls back to the default rather than sending "".
func TestEmbed_DefaultsTheModel(t *testing.T) {
	RegisterTestingT(t)

	rec := &capture{}
	srv := openAIServer(t, rec, func(body map[string]any) (int, any) {
		return 200, map[string]any{"data": []openAIData{{Index: 0, Embedding: []float32{0.1}}}}
	})

	cfg := EmbedConfig{Provider: "openai_compatible", BaseURL: srv.URL} // no Model
	_, err := Embed(context.Background(), cfg, []string{"a"})
	Expect(err).ToNot(HaveOccurred())
	Expect(rec.bodies[0]["model"]).To(Equal(DefaultEmbedModel))
}

// The model list is offered as a starting point, but the input stays free-text —
// so anything in the list must at least be a value Embed will send unchanged.
func TestEmbedModelOptions_AreNonEmpty(t *testing.T) {
	RegisterTestingT(t)

	Expect(EmbedModelOptions).ToNot(BeEmpty())
	for _, opt := range EmbedModelOptions {
		Expect(opt.Value).ToNot(BeEmpty())
		Expect(opt.Name).ToNot(BeEmpty())
	}
	Expect(DefaultEmbedModel).To(Equal("text-embedding-3-small"))
}
