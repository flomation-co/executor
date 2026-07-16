package ai_common

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

// Embeddings — turning text into the vector that represents its meaning.
//
// This is the capability the platform was missing. Embedding code did exist
// before this file, but it was private to actions/agent/recall and
// actions/agent/remember (a byte-for-byte duplicate), hardwired to Bedrock, and
// its result was never exposed as an action output — so no flow could ever get
// a vector out of one. A vector database with no way to produce a vector is a
// warehouse with no doors, which is why this ships alongside the pgvector node.
//
// Both the ai/embed_text action and the pgvector node's inline "embed the text
// for me" path call straight into Embed, so there is exactly one implementation
// of each provider's quirks.

const (
	// EmbedTimeout bounds one embedding call. Embedding is much cheaper than
	// generation, but a large batch against a cold self-hosted model is not
	// instant.
	EmbedTimeout = 120 * time.Second

	// maxEmbedResponse caps the response body. A 3072-dimension batch of 96 is
	// a few megabytes of JSON; 64MB is far beyond any legitimate reply and
	// bounds a hostile or malfunctioning one.
	maxEmbedResponse = 64 << 20

	// openAIBatchLimit is the number of inputs OpenAI accepts per request.
	openAIBatchLimit = 96

	// defaultAzureAPIVersion is Azure OpenAI's current GA data-plane version.
	// Every Azure call carries an api-version query param; this is used when
	// the caller left EmbedConfig.APIVersion empty.
	defaultAzureAPIVersion = "2024-10-21"
)

// EmbedProviderOptions are the providers the platform can embed with. They are
// shared by ai/embed_text and by every pgvector action's inline embedding
// block, so the two can never drift apart.
//
// OpenRouter is deliberately absent: it proxies chat completions only and has
// no embeddings endpoint.
var EmbedProviderOptions = []core.ConnectionOption{
	{Name: "OpenAI", Value: "openai"},
	{Name: "OpenAI-compatible (Azure, vLLM, LocalAI, TEI…)", Value: "openai_compatible"},
	{Name: "Azure OpenAI", Value: "azure_openai"},
	{Name: "Ollama (self-hosted)", Value: "ollama"},
	{Name: "AWS Bedrock (Titan)", Value: "bedrock"},
}

// EmbedModelOptions are the common models, offered as a starting point. The
// input stays free-text so a model we have never heard of still works.
var EmbedModelOptions = []core.ConnectionOption{
	{Name: "OpenAI text-embedding-3-small (1536 dimensions)", Value: "text-embedding-3-small"},
	{Name: "OpenAI text-embedding-3-large (3072 dimensions)", Value: "text-embedding-3-large"},
	{Name: "OpenAI text-embedding-ada-002 (1536 dimensions)", Value: "text-embedding-ada-002"},
	{Name: "Bedrock Titan Text v2 (1024 dimensions)", Value: "amazon.titan-embed-text-v2:0"},
	{Name: "Bedrock Titan Text v1 (1536 dimensions)", Value: "amazon.titan-embed-text-v1"},
	{Name: "Ollama nomic-embed-text (768 dimensions)", Value: "nomic-embed-text"},
	{Name: "Ollama mxbai-embed-large (1024 dimensions)", Value: "mxbai-embed-large"},
}

// DefaultEmbedModel is OpenAI's cheapest current model, and the 1536 dimensions
// it produces are what most pgvector tables in the wild are built for.
const DefaultEmbedModel = "text-embedding-3-small"

// EmbedConfig is one provider's worth of connection detail.
type EmbedConfig struct {
	Provider   string
	Model      string // for azure_openai this is the DEPLOYMENT name, not a model id
	BaseURL    string // openai_compatible, ollama, azure_openai (the resource endpoint)
	APIKey     string // openai, openai_compatible, azure_openai
	APIVersion string // azure_openai only — api-version query param; empty = defaultAzureAPIVersion
	Region     string // bedrock
	AccessKey  string // bedrock — static credentials, NOT the ambient chain
	SecretKey  string
	Dimensions int // 0 = the model's default
}

// embedClient is shared so connections are pooled across a batch.
//
// The per-request Timeout is a backstop, not the primary deadline: every call
// already carries a context deadline from the action (flow.GoContext derived, or
// EmbedTimeout when there is none), and that context is what actually bounds the
// request. The client Timeout only matters if a caller ever reaches Embed with a
// context that has no deadline of its own; it is kept so that path cannot hang.
var embedClient = &http.Client{Timeout: EmbedTimeout}

// Embed returns one vector per input text, in the same order.
func Embed(ctx context.Context, cfg EmbedConfig, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, errors.New("there is no text to embed")
	}
	for i, t := range texts {
		if strings.TrimSpace(t) == "" {
			return nil, fmt.Errorf("document %d has no text to embed", i+1)
		}
	}
	if cfg.Model == "" {
		cfg.Model = DefaultEmbedModel
	}

	switch cfg.Provider {
	case "", "openai":
		cfg.BaseURL = "https://api.openai.com"
		return embedOpenAI(ctx, cfg, texts)
	case "openai_compatible":
		if cfg.BaseURL == "" {
			return nil, errors.New("an OpenAI-compatible provider needs a Base URL")
		}
		return embedOpenAI(ctx, cfg, texts)
	case "azure_openai":
		if cfg.BaseURL == "" {
			return nil, errors.New(
				"Azure OpenAI needs the resource endpoint as Base URL (https://<resource>.openai.azure.com)")
		}
		return embedAzureOpenAI(ctx, cfg, texts)
	case "ollama":
		if cfg.BaseURL == "" {
			cfg.BaseURL = "http://localhost:11434"
		}
		return embedOllama(ctx, cfg, texts)
	case "bedrock":
		return embedBedrock(ctx, cfg, texts)
	}
	return nil, fmt.Errorf("%q isn't an embedding provider we support", cfg.Provider)
}

// RedactEmbed strips provider credentials out of a message.
func RedactEmbed(cfg EmbedConfig, msg string) string {
	for _, s := range []string{cfg.APIKey, cfg.SecretKey} {
		if s != "" {
			msg = strings.ReplaceAll(msg, s, "********")
		}
	}
	return msg
}

// ---------------------------------------------------------------------------
// OpenAI and OpenAI-compatible
// ---------------------------------------------------------------------------

func embedOpenAI(ctx context.Context, cfg EmbedConfig, texts []string) ([][]float32, error) {
	if cfg.APIKey == "" && cfg.Provider == "openai" {
		return nil, errors.New("OpenAI needs an API Key")
	}
	endpoint := strings.TrimSuffix(cfg.BaseURL, "/") + "/v1/embeddings"
	// Only text-embedding-3-* honour this; ada-002 rejects it outright.
	sendDims := cfg.Dimensions > 0 && strings.HasPrefix(cfg.Model, "text-embedding-3")
	return embedOpenAIWire(ctx, cfg, endpoint, sendDims, texts)
}

// embedAzureOpenAI speaks the OpenAI embeddings request/response shape against
// Azure's deployment-scoped URL. The two Azure deltas live elsewhere: the URL
// is built here (the model is the customer-named DEPLOYMENT, in the path, plus
// the mandatory api-version query param), and postJSON sends the key in the
// non-standard api-key header instead of Authorization: Bearer.
func embedAzureOpenAI(ctx context.Context, cfg EmbedConfig, texts []string) ([][]float32, error) {
	if cfg.APIKey == "" {
		return nil, errors.New("Azure OpenAI needs an API Key")
	}
	version := cfg.APIVersion
	if version == "" {
		version = defaultAzureAPIVersion
	}
	endpoint := strings.TrimSuffix(cfg.BaseURL, "/") +
		"/openai/deployments/" + url.PathEscape(cfg.Model) +
		"/embeddings?api-version=" + url.QueryEscape(version)
	// A deployment name says nothing about the model behind it, so the
	// text-embedding-3 prefix gate can't apply — pass dimensions through
	// whenever set. A deployment of a model that can't be resized rejects
	// the request, which is louder (and safer) than silently ignoring the
	// operator's requested size.
	return embedOpenAIWire(ctx, cfg, endpoint, cfg.Dimensions > 0, texts)
}

// embedOpenAIWire is the shared OpenAI-shape request loop: batches of at most
// openAIBatchLimit, reassembled by each reply's explicit index.
func embedOpenAIWire(ctx context.Context, cfg EmbedConfig, endpoint string, sendDims bool, texts []string) ([][]float32, error) {
	out := make([][]float32, 0, len(texts))
	// OpenAI caps the inputs per request, and a self-hosted compatible server
	// usually caps it lower still, so batch rather than assume.
	for start := 0; start < len(texts); start += openAIBatchLimit {
		end := start + openAIBatchLimit
		if end > len(texts) {
			end = len(texts)
		}
		chunk := texts[start:end]

		body := map[string]interface{}{
			"model": cfg.Model,
			"input": chunk,
		}
		if sendDims {
			body["dimensions"] = cfg.Dimensions
		}

		var resp struct {
			Data []struct {
				Index     int       `json:"index"`
				Embedding []float32 `json:"embedding"`
			} `json:"data"`
			Error *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := postJSON(ctx, cfg, endpoint, body, &resp); err != nil {
			return nil, err
		}
		if resp.Error != nil {
			return nil, errors.New(resp.Error.Message)
		}
		if len(resp.Data) != len(chunk) {
			return nil, fmt.Errorf(
				"asked the model for %d embeddings but got %d back", len(chunk), len(resp.Data))
		}

		// The API documents that data is returned in input order, but it also
		// carries an explicit index — trust the index.
		ordered := make([][]float32, len(chunk))
		for _, d := range resp.Data {
			if d.Index < 0 || d.Index >= len(chunk) {
				return nil, fmt.Errorf("the model returned an embedding for input %d, which wasn't sent", d.Index)
			}
			ordered[d.Index] = d.Embedding
		}
		for i, v := range ordered {
			if len(v) == 0 {
				return nil, fmt.Errorf("the model returned an empty embedding for document %d", start+i+1)
			}
		}
		out = append(out, ordered...)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Ollama
// ---------------------------------------------------------------------------

func embedOllama(ctx context.Context, cfg EmbedConfig, texts []string) ([][]float32, error) {
	// /api/embed is the batch endpoint and supersedes the older, single-input
	// /api/embeddings (which also spells its response key differently).
	endpoint := strings.TrimSuffix(cfg.BaseURL, "/") + "/api/embed"

	var resp struct {
		Embeddings [][]float32 `json:"embeddings"`
		Error      string      `json:"error"`
	}
	body := map[string]interface{}{
		"model": cfg.Model,
		"input": texts,
	}
	if err := postJSON(ctx, cfg, endpoint, body, &resp); err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, errors.New(resp.Error)
	}
	if len(resp.Embeddings) != len(texts) {
		return nil, fmt.Errorf(
			"asked Ollama for %d embeddings but got %d back — is %q an embedding model?",
			len(texts), len(resp.Embeddings), cfg.Model)
	}
	return resp.Embeddings, nil
}

// ---------------------------------------------------------------------------
// Bedrock
// ---------------------------------------------------------------------------

func embedBedrock(ctx context.Context, cfg EmbedConfig, texts []string) ([][]float32, error) {
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}

	opts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(region)}
	// Explicit keys when supplied, otherwise fall back to the ambient chain
	// (instance role, env, ~/.aws) so a flow running on EC2 needs no secrets.
	// agent/recall and agent/remember only ever had the ambient path, which is
	// why their semantic recall silently degrades on a self-hosted box.
	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("couldn't set up AWS credentials: %w", err)
	}
	client := bedrockruntime.NewFromConfig(awsCfg)

	contentType := "application/json"
	out := make([][]float32, len(texts))

	// Titan embeds one input per call — it has no batch endpoint. If the context
	// is cancelled part-way through (the flow was killed after text 3 of 50),
	// InvokeModel returns that error and this returns nil plus the error — never
	// the partially-filled `out`. That invariant is load-bearing: every caller
	// treats a non-nil error as a full abort, so a truncated batch can never be
	// mistaken for a complete one.
	//
	// The flip side of the serial loop is that a large batch shares one deadline,
	// so it can run out of time part-way through. The deadline case is caught
	// below and reported as what it is — a batch too big to finish — rather than
	// a bare "context deadline exceeded" that gives the operator nothing to act
	// on.
	for i, text := range texts {
		req := map[string]interface{}{"inputText": text, "normalize": true}
		if cfg.Dimensions > 0 {
			req["dimensions"] = cfg.Dimensions
		}
		raw, err := json.Marshal(req)
		if err != nil {
			return nil, err
		}

		model := cfg.Model
		resp, err := client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
			ModelId:     &model,
			Body:        raw,
			ContentType: &contentType,
			Accept:      &contentType,
		})
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, fmt.Errorf(
					"Bedrock ran out of time on text %d of %d — it embeds one text per request, so a large batch "+
						"can exceed the time limit. Split it into smaller batches (around 50 at a time)", i+1, len(texts))
			}
			return nil, fmt.Errorf("Bedrock rejected the request: %w", err)
		}

		var result struct {
			Embedding []float32 `json:"embedding"`
		}
		if err := json.Unmarshal(resp.Body, &result); err != nil {
			return nil, fmt.Errorf("couldn't read Bedrock's reply: %w", err)
		}
		if len(result.Embedding) == 0 {
			return nil, fmt.Errorf("Bedrock returned an empty embedding for document %d", i+1)
		}
		out[i] = result.Embedding
	}
	return out, nil
}

// ---------------------------------------------------------------------------

func postJSON(ctx context.Context, cfg EmbedConfig, endpoint string, body, into interface{}) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("%q isn't a usable URL: %w", endpoint, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		if cfg.Provider == "azure_openai" {
			// Azure's data plane wants the key in the non-standard api-key
			// header; it rejects Authorization: Bearer for key auth.
			req.Header.Set("api-key", cfg.APIKey)
		} else {
			req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
		}
	}

	resp, err := embedClient.Do(req)
	if err != nil {
		return errors.New(RedactEmbed(cfg, err.Error()))
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(io.LimitReader(resp.Body, maxEmbedResponse))
	if err != nil {
		return fmt.Errorf("couldn't read the reply from %s: %w", endpoint, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Prefer the provider's own message — "invalid api key" beats "HTTP 401".
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
			Message string `json:"message"`
		}
		msg := strings.TrimSpace(string(payload))
		// Best-effort parse: if the body isn't the JSON error shape we expect,
		// the Unmarshal failure is deliberately ignored and msg stays as the raw
		// body text set above — a readable fallback either way.
		if json.Unmarshal(payload, &e) == nil {
			if e.Error.Message != "" {
				msg = e.Error.Message
			} else if e.Message != "" {
				msg = e.Message
			}
		}
		if len(msg) > 500 {
			msg = msg[:500] + "…"
		}
		return fmt.Errorf("the embedding provider returned %d: %s",
			resp.StatusCode, RedactEmbed(cfg, msg))
	}

	if err := json.Unmarshal(payload, into); err != nil {
		return fmt.Errorf("couldn't read the reply from %s: %w", endpoint, err)
	}
	return nil
}
