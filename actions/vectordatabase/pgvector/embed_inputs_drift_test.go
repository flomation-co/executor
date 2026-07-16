package pgvector_test

import (
	"reflect"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/vectordatabase/pgvector"

	documentInsert "flomation.app/automate/executor/actions/vectordatabase/pgvector/document_insert"
	documentSearch "flomation.app/automate/executor/actions/vectordatabase/pgvector/document_search"
	documentUpdate "flomation.app/automate/executor/actions/vectordatabase/pgvector/document_update"
	documentUpsert "flomation.app/automate/executor/actions/vectordatabase/pgvector/document_upsert"
	hybridSearch "flomation.app/automate/executor/actions/vectordatabase/pgvector/hybrid_search"
)

// Every action that can embed inline re-declares pgvector.EmbedInputs as a
// literal, because the manifest generator AST-parses each action's literal
// Inputs array and cannot follow a shared variable. That makes the copies free
// to drift from the canonical block — and they did: when ai_common gained the
// azure_openai provider, EmbedInputs picked it up automatically (it references
// ai_common.EmbedProviderOptions) while all five copies silently kept the old
// four-provider list, so the backend supported Azure OpenAI but no operator
// could select it here.
//
// This test is the guard that was missing. It pins each copy to the canonical
// block, so the next provider added to ai_common fails here instead of
// shipping a dropdown that quietly omits it.
func TestEmbedInputsDoNotDriftFromCanonical(t *testing.T) {
	actions := map[string][]core.Connection{
		"document_insert": documentInsert.Inputs[:],
		"document_search": documentSearch.Inputs[:],
		"document_update": documentUpdate.Inputs[:],
		"document_upsert": documentUpsert.Inputs[:],
		"hybrid_search":   hybridSearch.Inputs[:],
	}

	for name, inputs := range actions {
		t.Run(name, func(t *testing.T) {
			byName := map[string]core.Connection{}
			for _, in := range inputs {
				byName[in.Name] = in
			}
			for _, want := range pgvector.EmbedInputs {
				got, ok := byName[want.Name]
				if !ok {
					t.Errorf("input %q is in pgvector.EmbedInputs but missing from this action", want.Name)
					continue
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("input %q has drifted from pgvector.EmbedInputs:\n got: %+v\nwant: %+v", want.Name, got, want)
				}
			}
		})
	}
}

// TestEveryEmbedProviderIsSelectable proves the dropdown offers every provider
// the shared Embed implementation can actually dispatch to — the exact
// invariant azure_openai broke.
func TestEveryEmbedProviderIsSelectable(t *testing.T) {
	var providerInput *core.Connection
	for i := range pgvector.EmbedInputs {
		if pgvector.EmbedInputs[i].Name == "provider" {
			providerInput = &pgvector.EmbedInputs[i]
			break
		}
	}
	if providerInput == nil {
		t.Fatal("pgvector.EmbedInputs has no provider input")
	}
	offered := map[string]bool{}
	for _, o := range providerInput.Options {
		offered[o.Value] = true
	}
	for _, want := range []string{"openai", "openai_compatible", "azure_openai", "ollama", "bedrock"} {
		if !offered[want] {
			t.Errorf("provider %q is dispatchable by ai_common.Embed but not offered in the pgvector dropdown", want)
		}
	}
}
