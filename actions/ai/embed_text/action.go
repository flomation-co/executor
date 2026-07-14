package ai_embed_text

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	ai_common "flomation.app/automate/executor/actions/ai"
)

// Embed Text is the door into the vector database.
//
// Before this action, nothing in the manifest could emit a vector: the embedding
// code existed but was locked inside agent/recall and agent/remember, hardwired
// to Bedrock, and never surfaced as an output. So a flow could store memories the
// agent used, and nothing else. This action exposes the same capability as a
// first-class step — one text (or a batch) in, a vector out — which is what makes
// "embed here, insert or search there" a flow an operator can actually build.
//
// Every provider quirk lives in ai_common.Embed; this file is only the operator's
// side of it.

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Embed Text"
	Description  = "Turn text into an embedding — the numeric fingerprint of its meaning, for storing in or searching a vector database"
	Website      = "https://www.flomation.co"
	Icon         = "brain+bolt"
	Date         = "13/07/2026"
	Type         = core.ActionTypeAction

	// maxTexts bounds one batch. Every vector is held in memory and echoed into
	// the outputs, so a 3072-dimension batch of thousands is measured in tens of
	// megabytes — and a batch that big is a mis-wired loop, not a real request.
	maxTexts = 1000
)

var Inputs = [...]core.Connection{
	{Name: "provider", Type: core.ConnectionTypeString, Label: "Embedding Provider", Required: true, Options: []core.ConnectionOption{{Name: "OpenAI", Value: "openai"}, {Name: "OpenAI-compatible (Azure, vLLM, LocalAI, TEI…)", Value: "openai_compatible"}, {Name: "Ollama (self-hosted)", Value: "ollama"}, {Name: "AWS Bedrock (Titan)", Value: "bedrock"}}},
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "API Key", Visible: &core.VisibleWhen{Field: "provider", Values: []string{"openai", "openai_compatible"}}},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "Base URL", Placeholder: "http://ollama.internal:11434", Visible: &core.VisibleWhen{Field: "provider", Values: []string{"openai_compatible", "ollama"}}},
	{Name: "access_key", Type: core.ConnectionTypeSecret, Label: "AWS Access Key ID", Visible: &core.VisibleWhen{Field: "provider", Values: []string{"bedrock"}}},
	{Name: "secret_key", Type: core.ConnectionTypeSecret, Label: "AWS Secret Access Key", Visible: &core.VisibleWhen{Field: "provider", Values: []string{"bedrock"}}},
	{Name: "aws_region", Type: core.ConnectionTypeString, Label: "AWS Region", Placeholder: "us-east-1", Visible: &core.VisibleWhen{Field: "provider", Values: []string{"bedrock"}}},
	{Name: "model", Type: core.ConnectionTypeComboBox, Label: "Model", Placeholder: "text-embedding-3-small", Required: true, Options: []core.ConnectionOption{{Name: "OpenAI text-embedding-3-small (1536 dimensions)", Value: "text-embedding-3-small"}, {Name: "OpenAI text-embedding-3-large (3072 dimensions)", Value: "text-embedding-3-large"}, {Name: "OpenAI text-embedding-ada-002 (1536 dimensions)", Value: "text-embedding-ada-002"}, {Name: "Bedrock Titan Text v2 (1024 dimensions)", Value: "amazon.titan-embed-text-v2:0"}, {Name: "Bedrock Titan Text v1 (1536 dimensions)", Value: "amazon.titan-embed-text-v1"}, {Name: "Ollama nomic-embed-text (768 dimensions)", Value: "nomic-embed-text"}, {Name: "Ollama mxbai-embed-large (1024 dimensions)", Value: "mxbai-embed-large"}}},
	{Name: "text", Type: core.ConnectionTypeText, Label: "Text", Placeholder: "The text to embed"},
	{Name: "texts", Type: core.ConnectionTypeObject, Label: "Texts (JSON array)", Placeholder: `Embed several at once: ["first", "second"] — overrides Text above`},
	{Name: "dimensions", Type: core.ConnectionTypeInteger, Label: "Dimensions", Placeholder: "Leave empty for the model's default. Must match your vector table (OpenAI 3-small = 1536, Bedrock Titan v2 = 1024)"},
}

var Outputs = [...]core.Connection{
	{Name: "embedding", Type: core.ConnectionTypeObject, Label: "Embedding"},
	{Name: "embeddings", Type: core.ConnectionTypeObject, Label: "Embeddings"},
	{Name: "dimensions", Type: core.ConnectionTypeInteger, Label: "Dimensions"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Texts Embedded"},
	{Name: "model", Type: core.ConnectionTypeString, Label: "Model Used"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	cfg := ai_common.EmbedConfig{
		Provider:   optString(core.FindConnection("provider", inputs)),
		Model:      optString(core.FindConnection("model", inputs)),
		BaseURL:    optString(core.FindConnection("base_url", inputs)),
		APIKey:     optString(core.FindConnection("api_key", inputs)),
		Region:     optString(core.FindConnection("aws_region", inputs)),
		AccessKey:  optString(core.FindConnection("access_key", inputs)),
		SecretKey:  optString(core.FindConnection("secret_key", inputs)),
		Dimensions: optInt(core.FindConnection("dimensions", inputs)),
	}
	if cfg.Provider == "" {
		cfg.Provider = "openai"
	}
	if cfg.Model == "" {
		cfg.Model = ai_common.DefaultEmbedModel
	}
	if cfg.Dimensions < 0 {
		return errorResult(fmt.Sprintf(
			"Dimensions is %d — leave it empty for the model's default, or set the size your vector table stores",
			cfg.Dimensions)), nil
	}

	texts, ignoredSingle, err := readTexts(inputs)
	if err != nil {
		return errorResult(err.Error()), nil
	}
	switch {
	case len(texts) == 0:
		return errorResult("There's nothing to embed — fill in Text, or give Texts a JSON array like [\"first\", \"second\"]"), nil
	case len(texts) > maxTexts:
		return errorResult(fmt.Sprintf(
			"Texts has %d entries, which is more than the %d this step embeds at once — split it across several runs",
			len(texts), maxTexts)), nil
	}

	// Embedding a cold self-hosted model, or a large batch, is not instant — but
	// it must still die with the run rather than outlive it.
	parent := context.Background()
	if flow != nil {
		if c := flow.GoContext(); c != nil {
			parent = c
		}
	}
	ctx, cancel := context.WithTimeout(parent, ai_common.EmbedTimeout)
	defer cancel()

	vectors, err := ai_common.Embed(ctx, cfg, texts)
	if err != nil {
		return errorResult(ai_common.RedactEmbed(cfg, err.Error())), nil
	}
	if len(vectors) == 0 || len(vectors[0]) == 0 {
		return errorResult(fmt.Sprintf("%s returned no embedding — is %q an embedding model?", providerLabel(cfg.Provider), cfg.Model)), nil
	}
	dimensions := len(vectors[0])

	summary := summarise(cfg, len(vectors), dimensions)
	if ignoredSingle {
		summary += " (The Text field was ignored because Texts was also filled in.)"
	}

	// The vectors go out as plain JSON number arrays, not wrapped in an object,
	// so the pgvector node's Embedding input reads them straight and a human can
	// see what came back.
	result := map[string]interface{}{
		"model":      cfg.Model,
		"provider":   cfg.Provider,
		"count":      len(vectors),
		"dimensions": dimensions,
		"embeddings": vectors,
	}

	return map[string]interface{}{
		"embedding":   vectors[0],
		"embeddings":  vectors,
		"dimensions":  dimensions,
		"count":       len(vectors),
		"model":       cfg.Model,
		"result":      result,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}, nil
}

// summarise is what the operator reads, and it deliberately does not contain the
// vector — that is thousands of floats nobody can act on. The dimension count,
// though, is the number they came for: it is how they find out their model makes
// 1536 and their table wants 1024, without running a failing insert first.
func summarise(cfg ai_common.EmbedConfig, count, dimensions int) string {
	noun := "text"
	if count != 1 {
		noun = "texts"
	}
	summary := fmt.Sprintf("Embedded %d %s with %s — %d dimensions.", count, noun, cfg.Model, dimensions)

	// Only OpenAI's text-embedding-3 models and Bedrock Titan v2 honour a
	// requested size; Ollama and ada-002 quietly ignore it. Say so, rather than
	// let the operator believe they got the size they asked for.
	if cfg.Dimensions > 0 && cfg.Dimensions != dimensions {
		summary += fmt.Sprintf(
			" You asked for %d dimensions, but %s ignored that and produced %d — this model can't be resized.",
			cfg.Dimensions, cfg.Model, dimensions)
	}
	return summary
}

func providerLabel(provider string) string {
	for _, o := range ai_common.EmbedProviderOptions {
		if o.Value == provider {
			return o.Name
		}
	}
	return "The embedding provider"
}

// readTexts resolves the two text inputs into the batch to embed. A non-empty
// Texts array wins: an operator who wired up a list has plainly moved on from
// the single-text field, and silently embedding the leftover Text instead would
// be the worst of both. It also reports whether a single Text was supplied but
// overridden that way, so Execute can say so — an operator who filled in both
// and got only the batch back would otherwise wonder where their single line
// went.
func readTexts(inputs []*core.Connection) (texts []string, ignoredSingle bool, err error) {
	batch, err := parseTexts(core.FindConnection("texts", inputs))
	if err != nil {
		return nil, false, err
	}
	single := optString(core.FindConnection("text", inputs))
	if len(batch) > 0 {
		return batch, single != "", nil
	}
	if single == "" {
		return nil, false, nil
	}
	return []string{single}, false, nil
}

// parseTexts accepts every shape the Texts input can arrive in: a real slice when
// it came straight from an upstream action in the same run, a []any of strings
// after a JSON round-trip through the flow store, or a JSON string when the
// ${...} reference passed through the substitution pass (which rewrites every
// reference into its resolved text).
func parseTexts(c *core.Connection) ([]string, error) {
	if c == nil || c.Value == nil {
		return nil, nil
	}

	switch v := c.Value.(type) {
	case []string:
		return v, nil

	case []any:
		out := make([]string, len(v))
		for i, e := range v {
			s, ok := e.(string)
			if !ok {
				return nil, fmt.Errorf(
					"entry %d of Texts is a %T, not a piece of text — Texts has to be a JSON array of strings, like [\"first\", \"second\"]",
					i+1, e)
			}
			out[i] = s
		}
		return out, nil

	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return nil, nil
		}
		var arr []string
		if err := json.Unmarshal([]byte(s), &arr); err != nil {
			return nil, errors.New(
				"Texts isn't a JSON array of strings — it should look like [\"first\", \"second\"]. " +
					"Use the Text box for a single piece of text")
		}
		return arr, nil
	}

	return nil, fmt.Errorf(
		"Texts is a %T — it should be a JSON array of strings, like [\"first\", \"second\"]", c.Value)
}

// optString reads a string/secret/text input, returning "" when it is unset.
//
// The nil-check is load-bearing for a Secret. Connection.String() special-cases
// the text-ish types and otherwise falls through to fmt.Sprintf("%v", …), so a
// secret that was never filled in stringifies to the literal text "<nil>" — which
// would then be sent to the provider as the API key.
func optString(c *core.Connection) string {
	if c == nil || c.Value == nil {
		return ""
	}
	s := c.String()
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}

// optInt reads an integer input, returning 0 when it is unset.
func optInt(c *core.Connection) int {
	if c == nil || c.Value == nil {
		return 0
	}
	if n := c.Number(); n != nil {
		return int(*n)
	}
	return 0
}

// errorResult is the soft-failure shape: returned with a nil error so the engine
// routes it to the error port as data, instead of killing the run.
func errorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}
}
