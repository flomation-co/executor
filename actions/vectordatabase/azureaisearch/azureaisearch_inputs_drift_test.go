// Cross-action invariants for the Vector Database ▸ Azure AI Search node.
//
// All 10 actions re-declare the four-field credential block INLINE, because
// the manifest generator AST-parses each action's Inputs literal and cannot
// see through a package-level variable. azureaisearch.AuthInputs is therefore
// documentation, not enforcement — 10 copies of four fields, free to drift
// one paste at a time. This file is the enforcement: a copy that drifts fails
// CI with the action and the field named. Modelled on
// actions/infrastructure/awx_inputs_drift_test.go.
package azureaisearch_test

import (
	"reflect"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	azureaisearch "flomation.app/automate/executor/actions/vectordatabase/azureaisearch"

	document_count "flomation.app/automate/executor/actions/vectordatabase/azureaisearch/document_count"
	document_delete "flomation.app/automate/executor/actions/vectordatabase/azureaisearch/document_delete"
	document_get "flomation.app/automate/executor/actions/vectordatabase/azureaisearch/document_get"
	document_upload "flomation.app/automate/executor/actions/vectordatabase/azureaisearch/document_upload"
	index_create "flomation.app/automate/executor/actions/vectordatabase/azureaisearch/index_create"
	index_delete "flomation.app/automate/executor/actions/vectordatabase/azureaisearch/index_delete"
	index_get "flomation.app/automate/executor/actions/vectordatabase/azureaisearch/index_get"
	index_get_all "flomation.app/automate/executor/actions/vectordatabase/azureaisearch/index_get_all"
	index_stats "flomation.app/automate/executor/actions/vectordatabase/azureaisearch/index_stats"
	search "flomation.app/automate/executor/actions/vectordatabase/azureaisearch/search"
)

// actionInputs is the table every assertion below ranges over. All 10 actions.
func actionInputs() map[string][]core.Connection {
	return map[string][]core.Connection{
		"azureaisearch/document_count":  document_count.Inputs[:],
		"azureaisearch/document_delete": document_delete.Inputs[:],
		"azureaisearch/document_get":    document_get.Inputs[:],
		"azureaisearch/document_upload": document_upload.Inputs[:],
		"azureaisearch/index_create":    index_create.Inputs[:],
		"azureaisearch/index_delete":    index_delete.Inputs[:],
		"azureaisearch/index_get":       index_get.Inputs[:],
		"azureaisearch/index_get_all":   index_get_all.Inputs[:],
		"azureaisearch/index_stats":     index_stats.Inputs[:],
		"azureaisearch/search":          search.Inputs[:],
	}
}

// actionOutputs backs the standard-outputs assertion.
func actionOutputs() map[string][]core.Connection {
	return map[string][]core.Connection{
		"azureaisearch/document_count":  document_count.Outputs[:],
		"azureaisearch/document_delete": document_delete.Outputs[:],
		"azureaisearch/document_get":    document_get.Outputs[:],
		"azureaisearch/document_upload": document_upload.Outputs[:],
		"azureaisearch/index_create":    index_create.Outputs[:],
		"azureaisearch/index_delete":    index_delete.Outputs[:],
		"azureaisearch/index_get":       index_get.Outputs[:],
		"azureaisearch/index_get_all":   index_get_all.Outputs[:],
		"azureaisearch/index_stats":     index_stats.Outputs[:],
		"azureaisearch/search":          search.Outputs[:],
	}
}

// actionIcons backs the icon-resolution assertion.
func actionIcons() map[string]string {
	return map[string]string{
		"azureaisearch/document_count":  document_count.Icon,
		"azureaisearch/document_delete": document_delete.Icon,
		"azureaisearch/document_get":    document_get.Icon,
		"azureaisearch/document_upload": document_upload.Icon,
		"azureaisearch/index_create":    index_create.Icon,
		"azureaisearch/index_delete":    index_delete.Icon,
		"azureaisearch/index_get":       index_get.Icon,
		"azureaisearch/index_get_all":   index_get_all.Icon,
		"azureaisearch/index_stats":     index_stats.Icon,
		"azureaisearch/search":          search.Icon,
	}
}

// knownBadges are the badge glyphs this node's icons use. Every name here was
// checked against editor/app/components/icons/paths.ts — a badge missing there
// renders as a silent "?" in the palette, which no compiler and no other test
// would catch. NOTE: "upload"/"download" do NOT exist in the editor's glyph
// set; arrow-up/arrow-down are the shipped equivalents.
var knownBadges = map[string]bool{
	"arrow-up":         true,
	"file":             true,
	"gauge":            true,
	"hashtag":          true,
	"list":             true,
	"magnifying-glass": true,
	"plus":             true,
	"trash":            true,
}

// TestAzureAISearchAuthBlockDoesNotDrift asserts every action's first four
// inputs reproduce azureaisearch.AuthInputs exactly — name, type, label,
// placeholder, required, in order. service_name and endpoint are deliberately
// NOT Required (either one suffices; GetAuth enforces at run time), while
// api_key is — a copy that flips either breaks the credential form.
func TestAzureAISearchAuthBlockDoesNotDrift(t *testing.T) {
	want := azureaisearch.AuthInputs

	for id, inputs := range actionInputs() {
		if len(inputs) < len(want) {
			t.Errorf("%s: has %d inputs, fewer than the %d-field credential block", id, len(inputs), len(want))
			continue
		}
		for i := range want {
			if !reflect.DeepEqual(inputs[i], want[i]) {
				t.Errorf("%s: credential input %d (%q) has drifted from azureaisearch.AuthInputs\n got: %+v\nwant: %+v",
					id, i, want[i].Name, inputs[i], want[i])
			}
		}
	}
}

// TestAzureAISearchNoResourceInputShadowsACredential guards the input-name
// collision core.FindConnection makes possible: it returns the FIRST input
// whose name matches, and the credential block is declared first — so a
// resource field reusing a credential's name silently reads the credential.
// "endpoint" and "api_version" are the plausible offenders here.
func TestAzureAISearchNoResourceInputShadowsACredential(t *testing.T) {
	credential := map[string]bool{}
	for _, c := range azureaisearch.AuthInputs {
		credential[c.Name] = true
	}

	for id, inputs := range actionInputs() {
		if len(inputs) < len(azureaisearch.AuthInputs) {
			continue // the drift test already reports this
		}
		for _, c := range inputs[len(azureaisearch.AuthInputs):] {
			if credential[c.Name] {
				t.Errorf("%s: resource input %q shadows the credential input of the same name — core.FindConnection would return the credential",
					id, c.Name)
			}
		}
	}
}

// TestAzureAISearchIconsResolve keeps every icon inside the glyph set the
// editor ships. All 10 actions wear the Vector Database circle-nodes base so
// the sub-group reads as one node in the palette (the sibling pgvector wears
// database+badge for the same reason).
func TestAzureAISearchIconsResolve(t *testing.T) {
	for id, icon := range actionIcons() {
		base, badge, composed := strings.Cut(icon, "+")
		if !composed {
			t.Errorf("%s: icon %q is not a base+badge composition", id, icon)
			continue
		}
		if base != "circle-nodes" {
			t.Errorf("%s: icon base is %q, not \"circle-nodes\"", id, base)
		}
		if !knownBadges[badge] {
			t.Errorf("%s: icon badge %q is not in the verified badge set — check editor/app/components/icons/paths.ts before adding it to knownBadges", id, badge)
		}
	}
}

// TestAzureAISearchStandardOutputsPresent pins the three outputs the platform
// depends on: success drives the soft-failure path, error carries the message,
// and tool_result is what the AI tool loop shows the model.
func TestAzureAISearchStandardOutputsPresent(t *testing.T) {
	for id, outputs := range actionOutputs() {
		have := map[string]bool{}
		for _, o := range outputs {
			have[o.Name] = true
		}
		for _, required := range []string{"success", "error", "tool_result"} {
			if !have[required] {
				t.Errorf("%s: missing the %q output", id, required)
			}
		}
	}
}

// TestAzureAISearchTableCoversEveryAction pins the designed action count so a
// later 11th action must be added to the tables in this file to pass CI.
func TestAzureAISearchTableCoversEveryAction(t *testing.T) {
	const designed = 10
	if got := len(actionInputs()); got != designed {
		t.Errorf("actionInputs() covers %d actions, expected %d — a new azureaisearch action must be added to the tables in this file", got, designed)
	}
	if got := len(actionOutputs()); got != designed {
		t.Errorf("actionOutputs() covers %d actions, expected %d", got, designed)
	}
	if got := len(actionIcons()); got != designed {
		t.Errorf("actionIcons() covers %d actions, expected %d", got, designed)
	}
}
