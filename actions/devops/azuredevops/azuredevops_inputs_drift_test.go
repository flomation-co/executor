// Cross-action invariants for the DevOps ▸ Azure DevOps node.
//
// All 33 devops/azuredevops actions re-declare the three-field credential block
// INLINE, because the manifest generator AST-parses the Inputs literal and
// cannot see through a package-level variable. azuredevops.AuthInputs is
// therefore documentation, not enforcement — 33 copies of three fields, free to
// drift one paste at a time. This file is the enforcement: a copy that drifts
// fails CI with the action and the field named. Modelled on
// actions/azure/storage/storage_inputs_drift_test.go.
package azuredevops_test

import (
	"reflect"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	azuredevops "flomation.app/automate/executor/actions/devops/azuredevops"

	branch_get_all "flomation.app/automate/executor/actions/devops/azuredevops/branch_get_all"
	build_cancel "flomation.app/automate/executor/actions/devops/azuredevops/build_cancel"
	build_get "flomation.app/automate/executor/actions/devops/azuredevops/build_get"
	build_get_all "flomation.app/automate/executor/actions/devops/azuredevops/build_get_all"
	build_log_get "flomation.app/automate/executor/actions/devops/azuredevops/build_log_get"
	commit_get_all "flomation.app/automate/executor/actions/devops/azuredevops/commit_get_all"
	pipeline_artifact_get "flomation.app/automate/executor/actions/devops/azuredevops/pipeline_artifact_get"
	pipeline_get_all "flomation.app/automate/executor/actions/devops/azuredevops/pipeline_get_all"
	pipeline_run "flomation.app/automate/executor/actions/devops/azuredevops/pipeline_run"
	pipeline_run_get "flomation.app/automate/executor/actions/devops/azuredevops/pipeline_run_get"
	pipeline_run_get_all "flomation.app/automate/executor/actions/devops/azuredevops/pipeline_run_get_all"
	pr_comment_add "flomation.app/automate/executor/actions/devops/azuredevops/pr_comment_add"
	pr_complete "flomation.app/automate/executor/actions/devops/azuredevops/pr_complete"
	pr_create "flomation.app/automate/executor/actions/devops/azuredevops/pr_create"
	pr_get "flomation.app/automate/executor/actions/devops/azuredevops/pr_get"
	pr_get_all "flomation.app/automate/executor/actions/devops/azuredevops/pr_get_all"
	pr_update "flomation.app/automate/executor/actions/devops/azuredevops/pr_update"
	project_get "flomation.app/automate/executor/actions/devops/azuredevops/project_get"
	project_get_all "flomation.app/automate/executor/actions/devops/azuredevops/project_get_all"
	release_create "flomation.app/automate/executor/actions/devops/azuredevops/release_create"
	release_get_all "flomation.app/automate/executor/actions/devops/azuredevops/release_get_all"
	repo_get "flomation.app/automate/executor/actions/devops/azuredevops/repo_get"
	repo_get_all "flomation.app/automate/executor/actions/devops/azuredevops/repo_get_all"
	team_get_all "flomation.app/automate/executor/actions/devops/azuredevops/team_get_all"
	workitem_comment_add "flomation.app/automate/executor/actions/devops/azuredevops/workitem_comment_add"
	workitem_comment_get_all "flomation.app/automate/executor/actions/devops/azuredevops/workitem_comment_get_all"
	workitem_create "flomation.app/automate/executor/actions/devops/azuredevops/workitem_create"
	workitem_delete "flomation.app/automate/executor/actions/devops/azuredevops/workitem_delete"
	workitem_get "flomation.app/automate/executor/actions/devops/azuredevops/workitem_get"
	workitem_get_batch "flomation.app/automate/executor/actions/devops/azuredevops/workitem_get_batch"
	workitem_query_wiql "flomation.app/automate/executor/actions/devops/azuredevops/workitem_query_wiql"
	workitem_type_get_all "flomation.app/automate/executor/actions/devops/azuredevops/workitem_type_get_all"
	workitem_update "flomation.app/automate/executor/actions/devops/azuredevops/workitem_update"
)

// adoActionInputs is the table every assertion below ranges over. All 33
// actions.
func adoActionInputs() map[string][]core.Connection {
	return map[string][]core.Connection{
		"devops/azuredevops/project_get_all":          project_get_all.Inputs[:],
		"devops/azuredevops/project_get":              project_get.Inputs[:],
		"devops/azuredevops/team_get_all":             team_get_all.Inputs[:],
		"devops/azuredevops/pipeline_get_all":         pipeline_get_all.Inputs[:],
		"devops/azuredevops/pipeline_run":             pipeline_run.Inputs[:],
		"devops/azuredevops/pipeline_run_get":         pipeline_run_get.Inputs[:],
		"devops/azuredevops/pipeline_run_get_all":     pipeline_run_get_all.Inputs[:],
		"devops/azuredevops/pipeline_artifact_get":    pipeline_artifact_get.Inputs[:],
		"devops/azuredevops/build_get_all":            build_get_all.Inputs[:],
		"devops/azuredevops/build_get":                build_get.Inputs[:],
		"devops/azuredevops/build_cancel":             build_cancel.Inputs[:],
		"devops/azuredevops/build_log_get":            build_log_get.Inputs[:],
		"devops/azuredevops/workitem_create":          workitem_create.Inputs[:],
		"devops/azuredevops/workitem_get":             workitem_get.Inputs[:],
		"devops/azuredevops/workitem_update":          workitem_update.Inputs[:],
		"devops/azuredevops/workitem_query_wiql":      workitem_query_wiql.Inputs[:],
		"devops/azuredevops/workitem_get_batch":       workitem_get_batch.Inputs[:],
		"devops/azuredevops/workitem_comment_add":     workitem_comment_add.Inputs[:],
		"devops/azuredevops/workitem_comment_get_all": workitem_comment_get_all.Inputs[:],
		"devops/azuredevops/workitem_type_get_all":    workitem_type_get_all.Inputs[:],
		"devops/azuredevops/workitem_delete":          workitem_delete.Inputs[:],
		"devops/azuredevops/repo_get_all":             repo_get_all.Inputs[:],
		"devops/azuredevops/repo_get":                 repo_get.Inputs[:],
		"devops/azuredevops/pr_create":                pr_create.Inputs[:],
		"devops/azuredevops/pr_get_all":               pr_get_all.Inputs[:],
		"devops/azuredevops/pr_get":                   pr_get.Inputs[:],
		"devops/azuredevops/pr_complete":              pr_complete.Inputs[:],
		"devops/azuredevops/pr_update":                pr_update.Inputs[:],
		"devops/azuredevops/pr_comment_add":           pr_comment_add.Inputs[:],
		"devops/azuredevops/commit_get_all":           commit_get_all.Inputs[:],
		"devops/azuredevops/branch_get_all":           branch_get_all.Inputs[:],
		"devops/azuredevops/release_get_all":          release_get_all.Inputs[:],
		"devops/azuredevops/release_create":           release_create.Inputs[:],
	}
}

// adoActionOutputs backs the standard-outputs assertion.
func adoActionOutputs() map[string][]core.Connection {
	return map[string][]core.Connection{
		"devops/azuredevops/project_get_all":          project_get_all.Outputs[:],
		"devops/azuredevops/project_get":              project_get.Outputs[:],
		"devops/azuredevops/team_get_all":             team_get_all.Outputs[:],
		"devops/azuredevops/pipeline_get_all":         pipeline_get_all.Outputs[:],
		"devops/azuredevops/pipeline_run":             pipeline_run.Outputs[:],
		"devops/azuredevops/pipeline_run_get":         pipeline_run_get.Outputs[:],
		"devops/azuredevops/pipeline_run_get_all":     pipeline_run_get_all.Outputs[:],
		"devops/azuredevops/pipeline_artifact_get":    pipeline_artifact_get.Outputs[:],
		"devops/azuredevops/build_get_all":            build_get_all.Outputs[:],
		"devops/azuredevops/build_get":                build_get.Outputs[:],
		"devops/azuredevops/build_cancel":             build_cancel.Outputs[:],
		"devops/azuredevops/build_log_get":            build_log_get.Outputs[:],
		"devops/azuredevops/workitem_create":          workitem_create.Outputs[:],
		"devops/azuredevops/workitem_get":             workitem_get.Outputs[:],
		"devops/azuredevops/workitem_update":          workitem_update.Outputs[:],
		"devops/azuredevops/workitem_query_wiql":      workitem_query_wiql.Outputs[:],
		"devops/azuredevops/workitem_get_batch":       workitem_get_batch.Outputs[:],
		"devops/azuredevops/workitem_comment_add":     workitem_comment_add.Outputs[:],
		"devops/azuredevops/workitem_comment_get_all": workitem_comment_get_all.Outputs[:],
		"devops/azuredevops/workitem_type_get_all":    workitem_type_get_all.Outputs[:],
		"devops/azuredevops/workitem_delete":          workitem_delete.Outputs[:],
		"devops/azuredevops/repo_get_all":             repo_get_all.Outputs[:],
		"devops/azuredevops/repo_get":                 repo_get.Outputs[:],
		"devops/azuredevops/pr_create":                pr_create.Outputs[:],
		"devops/azuredevops/pr_get_all":               pr_get_all.Outputs[:],
		"devops/azuredevops/pr_get":                   pr_get.Outputs[:],
		"devops/azuredevops/pr_complete":              pr_complete.Outputs[:],
		"devops/azuredevops/pr_update":                pr_update.Outputs[:],
		"devops/azuredevops/pr_comment_add":           pr_comment_add.Outputs[:],
		"devops/azuredevops/commit_get_all":           commit_get_all.Outputs[:],
		"devops/azuredevops/branch_get_all":           branch_get_all.Outputs[:],
		"devops/azuredevops/release_get_all":          release_get_all.Outputs[:],
		"devops/azuredevops/release_create":           release_create.Outputs[:],
	}
}

// adoActionIcons backs the icon-resolution assertion.
func adoActionIcons() map[string]string {
	return map[string]string{
		"devops/azuredevops/project_get_all":          project_get_all.Icon,
		"devops/azuredevops/project_get":              project_get.Icon,
		"devops/azuredevops/team_get_all":             team_get_all.Icon,
		"devops/azuredevops/pipeline_get_all":         pipeline_get_all.Icon,
		"devops/azuredevops/pipeline_run":             pipeline_run.Icon,
		"devops/azuredevops/pipeline_run_get":         pipeline_run_get.Icon,
		"devops/azuredevops/pipeline_run_get_all":     pipeline_run_get_all.Icon,
		"devops/azuredevops/pipeline_artifact_get":    pipeline_artifact_get.Icon,
		"devops/azuredevops/build_get_all":            build_get_all.Icon,
		"devops/azuredevops/build_get":                build_get.Icon,
		"devops/azuredevops/build_cancel":             build_cancel.Icon,
		"devops/azuredevops/build_log_get":            build_log_get.Icon,
		"devops/azuredevops/workitem_create":          workitem_create.Icon,
		"devops/azuredevops/workitem_get":             workitem_get.Icon,
		"devops/azuredevops/workitem_update":          workitem_update.Icon,
		"devops/azuredevops/workitem_query_wiql":      workitem_query_wiql.Icon,
		"devops/azuredevops/workitem_get_batch":       workitem_get_batch.Icon,
		"devops/azuredevops/workitem_comment_add":     workitem_comment_add.Icon,
		"devops/azuredevops/workitem_comment_get_all": workitem_comment_get_all.Icon,
		"devops/azuredevops/workitem_type_get_all":    workitem_type_get_all.Icon,
		"devops/azuredevops/workitem_delete":          workitem_delete.Icon,
		"devops/azuredevops/repo_get_all":             repo_get_all.Icon,
		"devops/azuredevops/repo_get":                 repo_get.Icon,
		"devops/azuredevops/pr_create":                pr_create.Icon,
		"devops/azuredevops/pr_get_all":               pr_get_all.Icon,
		"devops/azuredevops/pr_get":                   pr_get.Icon,
		"devops/azuredevops/pr_complete":              pr_complete.Icon,
		"devops/azuredevops/pr_update":                pr_update.Icon,
		"devops/azuredevops/pr_comment_add":           pr_comment_add.Icon,
		"devops/azuredevops/commit_get_all":           commit_get_all.Icon,
		"devops/azuredevops/branch_get_all":           branch_get_all.Icon,
		"devops/azuredevops/release_get_all":          release_get_all.Icon,
		"devops/azuredevops/release_create":           release_create.Icon,
	}
}

// adoBadges is the badge glyph set this node's icons draw on. Every name here
// was checked against editor/app/components/icons/paths.ts — a badge missing
// there renders as a silent "?" in the palette, which no compiler and no other
// test would catch.
//
// The absences shaped the choices: paths.ts has no code-pull-request, no
// code-commit, no rocket and no download glyph, so the pull-request actions
// wear the generic verb badges (plus/pen/check/eye), commits wear code, and
// branches wear code-branch.
var adoBadges = map[string]bool{
	"box-archive":      true,
	"check":            true,
	"circle-stop":      true,
	"code":             true,
	"code-branch":      true,
	"comment":          true,
	"comments":         true,
	"eye":              true,
	"file-lines":       true,
	"layer-group":      true,
	"list":             true,
	"magnifying-glass": true,
	"pen":              true,
	"play":             true,
	"plus":             true,
	"trash":            true,
	"user-group":       true,
}

// adoListActions use results/count instead of id/result.
var adoListActions = map[string]bool{
	"devops/azuredevops/branch_get_all":           true,
	"devops/azuredevops/build_get_all":            true,
	"devops/azuredevops/commit_get_all":           true,
	"devops/azuredevops/pipeline_get_all":         true,
	"devops/azuredevops/pipeline_run_get_all":     true,
	"devops/azuredevops/pr_get_all":               true,
	"devops/azuredevops/project_get_all":          true,
	"devops/azuredevops/release_get_all":          true,
	"devops/azuredevops/repo_get_all":             true,
	"devops/azuredevops/team_get_all":             true,
	"devops/azuredevops/workitem_comment_get_all": true,
	"devops/azuredevops/workitem_get_batch":       true,
	"devops/azuredevops/workitem_query_wiql":      true,
	"devops/azuredevops/workitem_type_get_all":    true,
}

// TestAuthBlockDoesNotDrift is the whole reason this file exists.
//
// Every action's first three inputs must reproduce azuredevops.AuthInputs
// exactly — name, type, label, placeholder and required, in order. The api's
// dynamic-options params name these inputs positionally by name, so a drifted
// copy breaks a live dropdown as surely as it breaks auth.
func TestAuthBlockDoesNotDrift(t *testing.T) {
	want := azuredevops.AuthInputs

	for id, inputs := range adoActionInputs() {
		if len(inputs) < len(want) {
			t.Errorf("%s: has %d inputs, fewer than the %d-field credential block", id, len(inputs), len(want))
			continue
		}
		for i := range want {
			if !reflect.DeepEqual(inputs[i], want[i]) {
				t.Errorf("%s: credential input %d (%q) has drifted from azuredevops.AuthInputs\n got: %+v\nwant: %+v",
					id, i, want[i].Name, inputs[i], want[i])
			}
		}
	}
}

// TestNoResourceInputShadowsACredential guards the input-name collision
// core.FindConnection makes possible: it returns the FIRST input whose name
// matches, and the credential block is declared first — so a resource field
// that reuses a credential's name silently reads the CREDENTIAL instead.
//
// The live trap here is api_version: an action wanting to expose a "version"
// concept must not name it that.
func TestNoResourceInputShadowsACredential(t *testing.T) {
	credential := map[string]bool{}
	for _, c := range azuredevops.AuthInputs {
		credential[c.Name] = true
	}

	for id, inputs := range adoActionInputs() {
		if len(inputs) < len(azuredevops.AuthInputs) {
			continue // TestAuthBlockDoesNotDrift already reports this
		}
		for _, c := range inputs[len(azuredevops.AuthInputs):] {
			if credential[c.Name] {
				t.Errorf("%s: resource input %q shadows the credential input of the same name — "+
					"core.FindConnection would return the credential; rename it", id, c.Name)
			}
		}
	}
}

// TestIconsResolve keeps every icon inside the glyph set the editor actually
// ships: the azure base plus a badge from adoBadges.
func TestIconsResolve(t *testing.T) {
	for id, icon := range adoActionIcons() {
		base, badge, composed := strings.Cut(icon, "+")
		if !composed {
			t.Errorf("%s: icon %q is not a base+badge composition", id, icon)
			continue
		}
		if base != "azure" {
			t.Errorf("%s: icon base is %q, not \"azure\" — every action in this node wears the Azure mark", id, base)
		}
		if !adoBadges[badge] {
			t.Errorf("%s: icon badge %q is not in the verified paths.ts glyph set — it would render as \"?\" in the palette", id, badge)
		}
	}
}

// TestStandardOutputsPresent pins the outputs the platform depends on: success
// drives the soft-failure path, error carries the message, tool_result is what
// the AI tool loop shows the model — plus the id/result vs results/count
// baseline split between single-resource and list actions.
func TestStandardOutputsPresent(t *testing.T) {
	for id, outputs := range adoActionOutputs() {
		have := map[string]bool{}
		for _, o := range outputs {
			have[o.Name] = true
		}
		for _, required := range []string{"success", "error", "tool_result"} {
			if !have[required] {
				t.Errorf("%s: missing the %q output", id, required)
			}
		}
		if adoListActions[id] {
			if !have["results"] || !have["count"] {
				t.Errorf("%s: list actions carry results + count", id)
			}
		} else if !have["id"] || !have["result"] {
			t.Errorf("%s: resource actions carry id + result", id)
		}
	}
}

// TestNoOutputShadowsTheBaseline pins a collision the compiler cannot see: an
// action lifting a field called "result" or "id" onto its output would clobber
// the baseline object. It is why the pipeline/build actions expose run_result
// and build_result rather than the API's own field name.
func TestNoOutputShadowsTheBaseline(t *testing.T) {
	for id, outputs := range adoActionOutputs() {
		seen := map[string]int{}
		for _, o := range outputs {
			seen[o.Name]++
			if seen[o.Name] > 1 {
				t.Errorf("%s: output %q is declared twice", id, o.Name)
			}
		}
	}
}

// TestTableCoversEveryActionOnDisk pins the designed action count. If action 34
// lands and nobody adds it to the tables here, this is what says so.
func TestTableCoversEveryActionOnDisk(t *testing.T) {
	const designed = 33
	if got := len(adoActionInputs()); got != designed {
		t.Errorf("adoActionInputs() covers %d actions, expected %d — a new action must be added to the tables in this file", got, designed)
	}
	if got := len(adoActionOutputs()); got != designed {
		t.Errorf("adoActionOutputs() covers %d actions, expected %d", got, designed)
	}
	if got := len(adoActionIcons()); got != designed {
		t.Errorf("adoActionIcons() covers %d actions, expected %d", got, designed)
	}
}
