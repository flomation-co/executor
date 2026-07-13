package pgvector

import (
	"context"
	"errors"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	ai_common "flomation.app/automate/executor/actions/ai"
)

// The embedding block: how a pgvector action gets a vector.
//
// n8n solves this by making its PGVector node a LangChain sub-node that you
// wire an Embeddings sub-node into. Flomation has no sub-nodes, and the
// front-of-house operator this product is for should not have to assemble two
// nodes to save a paragraph of text. So every action that needs a vector offers
// two routes and defaults to the simple one:
//
//   - "Embed the text for me" (default) — the action calls the embedding
//     provider itself. One node, one job, works on its own.
//   - "Use a vector from a previous step" — the action takes a ready-made
//     vector. This is the path for a pre-computed corpus, a fine-tuned model,
//     or reusing one embedding across several steps without paying to compute
//     it twice.
//
// EmbedInputs below is the canonical block, re-declared verbatim in each
// action's Inputs literal (the manifest generator AST-parses those, so it
// cannot follow a shared var) and pinned by inputs_drift_test.go.

// EmbedSourceOptions are the two routes.
var EmbedSourceOptions = []core.ConnectionOption{
	{Name: "Embed the text for me", Value: "inline"},
	{Name: "Use a vector from a previous step", Value: "vector"},
}

// EmbedInputs documents the canonical embedding block.
//
// The names here are chosen to avoid two separate collisions. First,
// core.FindConnection returns the FIRST input matching a name, and the
// connection block is declared before this one — so nothing here may be called
// host/port/database/username/password/schema/table. Second, api_key,
// access_key and secret_key are on the flow engine's credential strip-list, so
// naming them anything else would leak them into the tool schema handed to an
// LLM when the node runs as an agent tool.
var EmbedInputs = []core.Connection{
	{Name: "embedding_source", Type: core.ConnectionTypeString, Label: "Embedding Source", Required: true, Options: EmbedSourceOptions},
	{Name: "embedding", Type: core.ConnectionTypeObject, Label: "Embedding Vector", Placeholder: "Pick the Embedding output of an Embed Text step", Visible: &core.VisibleWhen{Field: "embedding_source", Values: []string{"vector"}}},
	{Name: "provider", Type: core.ConnectionTypeString, Label: "Embedding Provider", Required: true, Options: ai_common.EmbedProviderOptions, Visible: &core.VisibleWhen{Field: "embedding_source", Values: []string{"inline"}}},
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Embedding API Key", Visible: &core.VisibleWhen{Field: "provider", Values: []string{"openai", "openai_compatible"}}},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "Embedding Base URL", Placeholder: "http://ollama.internal:11434", Visible: &core.VisibleWhen{Field: "provider", Values: []string{"openai_compatible", "ollama"}}},
	{Name: "access_key", Type: core.ConnectionTypeSecret, Label: "AWS Access Key ID", Visible: &core.VisibleWhen{Field: "provider", Values: []string{"bedrock"}}},
	{Name: "secret_key", Type: core.ConnectionTypeSecret, Label: "AWS Secret Access Key", Visible: &core.VisibleWhen{Field: "provider", Values: []string{"bedrock"}}},
	{Name: "aws_region", Type: core.ConnectionTypeString, Label: "AWS Region", Placeholder: "us-east-1", Visible: &core.VisibleWhen{Field: "provider", Values: []string{"bedrock"}}},
	{Name: "model", Type: core.ConnectionTypeComboBox, Label: "Embedding Model", Placeholder: "text-embedding-3-small", Options: ai_common.EmbedModelOptions, Visible: &core.VisibleWhen{Field: "embedding_source", Values: []string{"inline"}}},
	{Name: "dimensions", Type: core.ConnectionTypeInteger, Label: "Dimensions", Placeholder: "Leave empty for the model's default — must match the table", Visible: &core.VisibleWhen{Field: "embedding_source", Values: []string{"inline"}}},
}

// EmbedSpec is the embedding block as the operator filled it in.
type EmbedSpec struct {
	Source string // "inline" | "vector"
	Config ai_common.EmbedConfig
	Vector []float32 // populated when Source == "vector"
}

// GetEmbedSpec reads the embedding block.
func GetEmbedSpec(inputs []*core.Connection) (EmbedSpec, error) {
	spec := EmbedSpec{Source: OptionalString(core.FindConnection("embedding_source", inputs))}
	if spec.Source == "" {
		spec.Source = "inline"
	}

	switch spec.Source {
	case "vector":
		c := core.FindConnection("embedding", inputs)
		if c == nil || c.Value == nil {
			return spec, errors.New(
				"Embedding Source is set to \"Use a vector from a previous step\", but no Embedding Vector is " +
					"connected — pick the Embedding output of an Embed Text step, or switch to \"Embed the text for me\"")
		}
		vec, err := CoerceVector(c.Value)
		if err != nil {
			return spec, err
		}
		if len(vec) == 0 {
			return spec, errors.New("the Embedding Vector is empty")
		}
		spec.Vector = vec
		return spec, nil

	case "inline":
		spec.Config = ai_common.EmbedConfig{
			Provider:   OptionalString(core.FindConnection("provider", inputs)),
			Model:      OptionalString(core.FindConnection("model", inputs)),
			BaseURL:    OptionalString(core.FindConnection("base_url", inputs)),
			APIKey:     OptionalString(core.FindConnection("api_key", inputs)),
			Region:     OptionalString(core.FindConnection("aws_region", inputs)),
			AccessKey:  OptionalString(core.FindConnection("access_key", inputs)),
			SecretKey:  OptionalString(core.FindConnection("secret_key", inputs)),
			Dimensions: OptionalInt(core.FindConnection("dimensions", inputs), 0),
		}
		if spec.Config.Provider == "" {
			spec.Config.Provider = "openai"
		}
		if spec.Config.Model == "" {
			spec.Config.Model = ai_common.DefaultEmbedModel
		}
		return spec, nil
	}

	return spec, fmt.Errorf(
		"%q isn't a valid Embedding Source — use \"inline\" or \"vector\"", spec.Source)
}

// EmbedTexts resolves the spec into one vector per text.
//
// The supplied-vector route only ever carries a single vector, so asking it to
// embed a batch is a wiring mistake worth naming explicitly: it means the
// operator pointed a bulk insert at one hand-supplied embedding, which would
// silently store the same vector against every document.
func (s EmbedSpec) EmbedTexts(ctx context.Context, texts []string) ([][]float32, error) {
	if s.Source == "vector" {
		if len(texts) > 1 {
			return nil, fmt.Errorf(
				"you're inserting %d documents but supplied a single embedding vector — every document would end "+
					"up with the same embedding. Switch Embedding Source to \"Embed the text for me\", or give each "+
					"document its own \"embedding\" in the Documents list", len(texts))
		}
		return [][]float32{s.Vector}, nil
	}
	return ai_common.Embed(ctx, s.Config, texts)
}

// EmbedOne resolves the spec into a single vector — the search path.
func (s EmbedSpec) EmbedOne(ctx context.Context, text string) ([]float32, error) {
	if s.Source == "vector" {
		return s.Vector, nil
	}
	if strings.TrimSpace(text) == "" {
		return nil, errors.New("there's no Search Query to embed")
	}
	vecs, err := ai_common.Embed(ctx, s.Config, []string{text})
	if err != nil {
		return nil, err
	}
	return vecs[0], nil
}

// EmbedError humanises a provider failure, stripping the API key out of it.
func (s EmbedSpec) EmbedError(err error) string {
	return ai_common.RedactEmbed(s.Config, err.Error())
}
