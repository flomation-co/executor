// Cross-action invariants for the Infrastructure ▸ AAP / AWX node.
//
// The sibling file in this package (inputs_drift_test.go) enforces the same class
// of invariants for Kubernetes and Helm. AWX gets its own file, on the precedent of
// actions/opentofu/inputs_drift_test.go, because almost nothing it asserts is
// shared: AWX has its own SEVEN-field credential block (not Kubernetes' eight), its
// own `ansible` icon base, and its own destructive set. Folding it into the tables
// next door would have meant a scope flag on every assertion.
//
// What this file is really for: all 59 AWX actions re-declare the credential block
// INLINE, because the manifest generator AST-parses the Inputs literal and cannot
// see through a package-level variable. awx.AuthInputs is therefore documentation,
// not enforcement — 59 copies of seven fields, free to drift one paste at a time.
// This is the enforcement. A copy that drifts fails CI with the action and the
// field named.
//
// THE TRIGGER IS ALMOST NOT HERE. The AWX notification-template trigger lives at
// actions/trigger/awx_webhook, as every Flomation trigger does. It registers as
// "trigger/awx_webhook", NOT "infrastructure/*" — so it is out of scope for the
// tables below (putting it in them would break TestEveryRegisteredActionIsCovered,
// which matches those tables against IDs prefixed "infrastructure/"), and it is not
// left uncovered by that test either, for the same reason. It carries no credential
// block at all: it RECEIVES a POST from AWX rather than calling out to it, so there
// is no auth block to drift and nothing for the shadowing test to check. Its
// echo/strip invariants belong beside it, in package trigger.
//
// The one thing worth pinning from here is its icon, because that is the single
// property it shares with the 59 actions — see TestAWXTriggerIconResolves.
package infrastructure_test

import (
	"reflect"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
	awx_trigger "flomation.app/automate/executor/actions/trigger/awx_webhook"

	awx_adhoc_command_get "flomation.app/automate/executor/actions/infrastructure/awx/adhoc_command_get"
	awx_adhoc_command_run "flomation.app/automate/executor/actions/infrastructure/awx/adhoc_command_run"
	awx_credential_create "flomation.app/automate/executor/actions/infrastructure/awx/credential_create"
	awx_credential_delete "flomation.app/automate/executor/actions/infrastructure/awx/credential_delete"
	awx_credential_get "flomation.app/automate/executor/actions/infrastructure/awx/credential_get"
	awx_credential_list "flomation.app/automate/executor/actions/infrastructure/awx/credential_list"
	awx_execution_environment_list "flomation.app/automate/executor/actions/infrastructure/awx/execution_environment_list"
	awx_group_create "flomation.app/automate/executor/actions/infrastructure/awx/group_create"
	awx_group_delete "flomation.app/automate/executor/actions/infrastructure/awx/group_delete"
	awx_group_get "flomation.app/automate/executor/actions/infrastructure/awx/group_get"
	awx_group_list "flomation.app/automate/executor/actions/infrastructure/awx/group_list"
	awx_group_update "flomation.app/automate/executor/actions/infrastructure/awx/group_update"
	awx_host_create "flomation.app/automate/executor/actions/infrastructure/awx/host_create"
	awx_host_delete "flomation.app/automate/executor/actions/infrastructure/awx/host_delete"
	awx_host_get "flomation.app/automate/executor/actions/infrastructure/awx/host_get"
	awx_host_group_assign "flomation.app/automate/executor/actions/infrastructure/awx/host_group_assign"
	awx_host_list "flomation.app/automate/executor/actions/infrastructure/awx/host_list"
	awx_host_update "flomation.app/automate/executor/actions/infrastructure/awx/host_update"
	awx_inventory_create "flomation.app/automate/executor/actions/infrastructure/awx/inventory_create"
	awx_inventory_delete "flomation.app/automate/executor/actions/infrastructure/awx/inventory_delete"
	awx_inventory_get "flomation.app/automate/executor/actions/infrastructure/awx/inventory_get"
	awx_inventory_list "flomation.app/automate/executor/actions/infrastructure/awx/inventory_list"
	awx_inventory_source_get "flomation.app/automate/executor/actions/infrastructure/awx/inventory_source_get"
	awx_inventory_source_list "flomation.app/automate/executor/actions/infrastructure/awx/inventory_source_list"
	awx_inventory_source_sync "flomation.app/automate/executor/actions/infrastructure/awx/inventory_source_sync"
	awx_inventory_update "flomation.app/automate/executor/actions/infrastructure/awx/inventory_update"
	awx_job_cancel "flomation.app/automate/executor/actions/infrastructure/awx/job_cancel"
	awx_job_events_list "flomation.app/automate/executor/actions/infrastructure/awx/job_events_list"
	awx_job_get "flomation.app/automate/executor/actions/infrastructure/awx/job_get"
	awx_job_list "flomation.app/automate/executor/actions/infrastructure/awx/job_list"
	awx_job_relaunch "flomation.app/automate/executor/actions/infrastructure/awx/job_relaunch"
	awx_job_stdout_get "flomation.app/automate/executor/actions/infrastructure/awx/job_stdout_get"
	awx_job_template_get "flomation.app/automate/executor/actions/infrastructure/awx/job_template_get"
	awx_job_template_launch "flomation.app/automate/executor/actions/infrastructure/awx/job_template_launch"
	awx_job_template_launch_options_get "flomation.app/automate/executor/actions/infrastructure/awx/job_template_launch_options_get"
	awx_job_template_list "flomation.app/automate/executor/actions/infrastructure/awx/job_template_list"
	awx_job_template_survey_get "flomation.app/automate/executor/actions/infrastructure/awx/job_template_survey_get"
	awx_job_wait "flomation.app/automate/executor/actions/infrastructure/awx/job_wait"
	awx_me "flomation.app/automate/executor/actions/infrastructure/awx/me"
	awx_organization_list "flomation.app/automate/executor/actions/infrastructure/awx/organization_list"
	awx_ping "flomation.app/automate/executor/actions/infrastructure/awx/ping"
	awx_project_create "flomation.app/automate/executor/actions/infrastructure/awx/project_create"
	awx_project_delete "flomation.app/automate/executor/actions/infrastructure/awx/project_delete"
	awx_project_get "flomation.app/automate/executor/actions/infrastructure/awx/project_get"
	awx_project_list "flomation.app/automate/executor/actions/infrastructure/awx/project_list"
	awx_project_sync "flomation.app/automate/executor/actions/infrastructure/awx/project_sync"
	awx_project_update "flomation.app/automate/executor/actions/infrastructure/awx/project_update"
	awx_schedule_create "flomation.app/automate/executor/actions/infrastructure/awx/schedule_create"
	awx_schedule_delete "flomation.app/automate/executor/actions/infrastructure/awx/schedule_delete"
	awx_schedule_list "flomation.app/automate/executor/actions/infrastructure/awx/schedule_list"
	awx_schedule_update "flomation.app/automate/executor/actions/infrastructure/awx/schedule_update"
	awx_team_list "flomation.app/automate/executor/actions/infrastructure/awx/team_list"
	awx_user_list "flomation.app/automate/executor/actions/infrastructure/awx/user_list"
	awx_workflow_job_get "flomation.app/automate/executor/actions/infrastructure/awx/workflow_job_get"
	awx_workflow_job_nodes_list "flomation.app/automate/executor/actions/infrastructure/awx/workflow_job_nodes_list"
	awx_workflow_job_relaunch "flomation.app/automate/executor/actions/infrastructure/awx/workflow_job_relaunch"
	awx_workflow_launch "flomation.app/automate/executor/actions/infrastructure/awx/workflow_launch"
	awx_workflow_template_get "flomation.app/automate/executor/actions/infrastructure/awx/workflow_template_get"
	awx_workflow_template_list "flomation.app/automate/executor/actions/infrastructure/awx/workflow_template_list"
)

// awxActionInputs is the table every assertion below ranges over. All 59 actions.
func awxActionInputs() map[string][]core.Connection {
	return map[string][]core.Connection{
		"awx/adhoc_command_get":               awx_adhoc_command_get.Inputs[:],
		"awx/adhoc_command_run":               awx_adhoc_command_run.Inputs[:],
		"awx/credential_create":               awx_credential_create.Inputs[:],
		"awx/credential_delete":               awx_credential_delete.Inputs[:],
		"awx/credential_get":                  awx_credential_get.Inputs[:],
		"awx/credential_list":                 awx_credential_list.Inputs[:],
		"awx/execution_environment_list":      awx_execution_environment_list.Inputs[:],
		"awx/group_create":                    awx_group_create.Inputs[:],
		"awx/group_delete":                    awx_group_delete.Inputs[:],
		"awx/group_get":                       awx_group_get.Inputs[:],
		"awx/group_list":                      awx_group_list.Inputs[:],
		"awx/group_update":                    awx_group_update.Inputs[:],
		"awx/host_create":                     awx_host_create.Inputs[:],
		"awx/host_delete":                     awx_host_delete.Inputs[:],
		"awx/host_get":                        awx_host_get.Inputs[:],
		"awx/host_group_assign":               awx_host_group_assign.Inputs[:],
		"awx/host_list":                       awx_host_list.Inputs[:],
		"awx/host_update":                     awx_host_update.Inputs[:],
		"awx/inventory_create":                awx_inventory_create.Inputs[:],
		"awx/inventory_delete":                awx_inventory_delete.Inputs[:],
		"awx/inventory_get":                   awx_inventory_get.Inputs[:],
		"awx/inventory_list":                  awx_inventory_list.Inputs[:],
		"awx/inventory_source_get":            awx_inventory_source_get.Inputs[:],
		"awx/inventory_source_list":           awx_inventory_source_list.Inputs[:],
		"awx/inventory_source_sync":           awx_inventory_source_sync.Inputs[:],
		"awx/inventory_update":                awx_inventory_update.Inputs[:],
		"awx/job_cancel":                      awx_job_cancel.Inputs[:],
		"awx/job_events_list":                 awx_job_events_list.Inputs[:],
		"awx/job_get":                         awx_job_get.Inputs[:],
		"awx/job_list":                        awx_job_list.Inputs[:],
		"awx/job_relaunch":                    awx_job_relaunch.Inputs[:],
		"awx/job_stdout_get":                  awx_job_stdout_get.Inputs[:],
		"awx/job_template_get":                awx_job_template_get.Inputs[:],
		"awx/job_template_launch":             awx_job_template_launch.Inputs[:],
		"awx/job_template_launch_options_get": awx_job_template_launch_options_get.Inputs[:],
		"awx/job_template_list":               awx_job_template_list.Inputs[:],
		"awx/job_template_survey_get":         awx_job_template_survey_get.Inputs[:],
		"awx/job_wait":                        awx_job_wait.Inputs[:],
		"awx/me":                              awx_me.Inputs[:],
		"awx/organization_list":               awx_organization_list.Inputs[:],
		"awx/ping":                            awx_ping.Inputs[:],
		"awx/project_create":                  awx_project_create.Inputs[:],
		"awx/project_delete":                  awx_project_delete.Inputs[:],
		"awx/project_get":                     awx_project_get.Inputs[:],
		"awx/project_list":                    awx_project_list.Inputs[:],
		"awx/project_sync":                    awx_project_sync.Inputs[:],
		"awx/project_update":                  awx_project_update.Inputs[:],
		"awx/schedule_create":                 awx_schedule_create.Inputs[:],
		"awx/schedule_delete":                 awx_schedule_delete.Inputs[:],
		"awx/schedule_list":                   awx_schedule_list.Inputs[:],
		"awx/schedule_update":                 awx_schedule_update.Inputs[:],
		"awx/team_list":                       awx_team_list.Inputs[:],
		"awx/user_list":                       awx_user_list.Inputs[:],
		"awx/workflow_job_get":                awx_workflow_job_get.Inputs[:],
		"awx/workflow_job_nodes_list":         awx_workflow_job_nodes_list.Inputs[:],
		"awx/workflow_job_relaunch":           awx_workflow_job_relaunch.Inputs[:],
		"awx/workflow_launch":                 awx_workflow_launch.Inputs[:],
		"awx/workflow_template_get":           awx_workflow_template_get.Inputs[:],
		"awx/workflow_template_list":          awx_workflow_template_list.Inputs[:],
	}
}

// awxActionOutputs backs the standard-outputs assertion.
func awxActionOutputs() map[string][]core.Connection {
	return map[string][]core.Connection{
		"awx/adhoc_command_get":               awx_adhoc_command_get.Outputs[:],
		"awx/adhoc_command_run":               awx_adhoc_command_run.Outputs[:],
		"awx/credential_create":               awx_credential_create.Outputs[:],
		"awx/credential_delete":               awx_credential_delete.Outputs[:],
		"awx/credential_get":                  awx_credential_get.Outputs[:],
		"awx/credential_list":                 awx_credential_list.Outputs[:],
		"awx/execution_environment_list":      awx_execution_environment_list.Outputs[:],
		"awx/group_create":                    awx_group_create.Outputs[:],
		"awx/group_delete":                    awx_group_delete.Outputs[:],
		"awx/group_get":                       awx_group_get.Outputs[:],
		"awx/group_list":                      awx_group_list.Outputs[:],
		"awx/group_update":                    awx_group_update.Outputs[:],
		"awx/host_create":                     awx_host_create.Outputs[:],
		"awx/host_delete":                     awx_host_delete.Outputs[:],
		"awx/host_get":                        awx_host_get.Outputs[:],
		"awx/host_group_assign":               awx_host_group_assign.Outputs[:],
		"awx/host_list":                       awx_host_list.Outputs[:],
		"awx/host_update":                     awx_host_update.Outputs[:],
		"awx/inventory_create":                awx_inventory_create.Outputs[:],
		"awx/inventory_delete":                awx_inventory_delete.Outputs[:],
		"awx/inventory_get":                   awx_inventory_get.Outputs[:],
		"awx/inventory_list":                  awx_inventory_list.Outputs[:],
		"awx/inventory_source_get":            awx_inventory_source_get.Outputs[:],
		"awx/inventory_source_list":           awx_inventory_source_list.Outputs[:],
		"awx/inventory_source_sync":           awx_inventory_source_sync.Outputs[:],
		"awx/inventory_update":                awx_inventory_update.Outputs[:],
		"awx/job_cancel":                      awx_job_cancel.Outputs[:],
		"awx/job_events_list":                 awx_job_events_list.Outputs[:],
		"awx/job_get":                         awx_job_get.Outputs[:],
		"awx/job_list":                        awx_job_list.Outputs[:],
		"awx/job_relaunch":                    awx_job_relaunch.Outputs[:],
		"awx/job_stdout_get":                  awx_job_stdout_get.Outputs[:],
		"awx/job_template_get":                awx_job_template_get.Outputs[:],
		"awx/job_template_launch":             awx_job_template_launch.Outputs[:],
		"awx/job_template_launch_options_get": awx_job_template_launch_options_get.Outputs[:],
		"awx/job_template_list":               awx_job_template_list.Outputs[:],
		"awx/job_template_survey_get":         awx_job_template_survey_get.Outputs[:],
		"awx/job_wait":                        awx_job_wait.Outputs[:],
		"awx/me":                              awx_me.Outputs[:],
		"awx/organization_list":               awx_organization_list.Outputs[:],
		"awx/ping":                            awx_ping.Outputs[:],
		"awx/project_create":                  awx_project_create.Outputs[:],
		"awx/project_delete":                  awx_project_delete.Outputs[:],
		"awx/project_get":                     awx_project_get.Outputs[:],
		"awx/project_list":                    awx_project_list.Outputs[:],
		"awx/project_sync":                    awx_project_sync.Outputs[:],
		"awx/project_update":                  awx_project_update.Outputs[:],
		"awx/schedule_create":                 awx_schedule_create.Outputs[:],
		"awx/schedule_delete":                 awx_schedule_delete.Outputs[:],
		"awx/schedule_list":                   awx_schedule_list.Outputs[:],
		"awx/schedule_update":                 awx_schedule_update.Outputs[:],
		"awx/team_list":                       awx_team_list.Outputs[:],
		"awx/user_list":                       awx_user_list.Outputs[:],
		"awx/workflow_job_get":                awx_workflow_job_get.Outputs[:],
		"awx/workflow_job_nodes_list":         awx_workflow_job_nodes_list.Outputs[:],
		"awx/workflow_job_relaunch":           awx_workflow_job_relaunch.Outputs[:],
		"awx/workflow_launch":                 awx_workflow_launch.Outputs[:],
		"awx/workflow_template_get":           awx_workflow_template_get.Outputs[:],
		"awx/workflow_template_list":          awx_workflow_template_list.Outputs[:],
	}
}

// awxActionIcons backs the icon-resolution assertion.
func awxActionIcons() map[string]string {
	return map[string]string{
		"awx/adhoc_command_get":               awx_adhoc_command_get.Icon,
		"awx/adhoc_command_run":               awx_adhoc_command_run.Icon,
		"awx/credential_create":               awx_credential_create.Icon,
		"awx/credential_delete":               awx_credential_delete.Icon,
		"awx/credential_get":                  awx_credential_get.Icon,
		"awx/credential_list":                 awx_credential_list.Icon,
		"awx/execution_environment_list":      awx_execution_environment_list.Icon,
		"awx/group_create":                    awx_group_create.Icon,
		"awx/group_delete":                    awx_group_delete.Icon,
		"awx/group_get":                       awx_group_get.Icon,
		"awx/group_list":                      awx_group_list.Icon,
		"awx/group_update":                    awx_group_update.Icon,
		"awx/host_create":                     awx_host_create.Icon,
		"awx/host_delete":                     awx_host_delete.Icon,
		"awx/host_get":                        awx_host_get.Icon,
		"awx/host_group_assign":               awx_host_group_assign.Icon,
		"awx/host_list":                       awx_host_list.Icon,
		"awx/host_update":                     awx_host_update.Icon,
		"awx/inventory_create":                awx_inventory_create.Icon,
		"awx/inventory_delete":                awx_inventory_delete.Icon,
		"awx/inventory_get":                   awx_inventory_get.Icon,
		"awx/inventory_list":                  awx_inventory_list.Icon,
		"awx/inventory_source_get":            awx_inventory_source_get.Icon,
		"awx/inventory_source_list":           awx_inventory_source_list.Icon,
		"awx/inventory_source_sync":           awx_inventory_source_sync.Icon,
		"awx/inventory_update":                awx_inventory_update.Icon,
		"awx/job_cancel":                      awx_job_cancel.Icon,
		"awx/job_events_list":                 awx_job_events_list.Icon,
		"awx/job_get":                         awx_job_get.Icon,
		"awx/job_list":                        awx_job_list.Icon,
		"awx/job_relaunch":                    awx_job_relaunch.Icon,
		"awx/job_stdout_get":                  awx_job_stdout_get.Icon,
		"awx/job_template_get":                awx_job_template_get.Icon,
		"awx/job_template_launch":             awx_job_template_launch.Icon,
		"awx/job_template_launch_options_get": awx_job_template_launch_options_get.Icon,
		"awx/job_template_list":               awx_job_template_list.Icon,
		"awx/job_template_survey_get":         awx_job_template_survey_get.Icon,
		"awx/job_wait":                        awx_job_wait.Icon,
		"awx/me":                              awx_me.Icon,
		"awx/organization_list":               awx_organization_list.Icon,
		"awx/ping":                            awx_ping.Icon,
		"awx/project_create":                  awx_project_create.Icon,
		"awx/project_delete":                  awx_project_delete.Icon,
		"awx/project_get":                     awx_project_get.Icon,
		"awx/project_list":                    awx_project_list.Icon,
		"awx/project_sync":                    awx_project_sync.Icon,
		"awx/project_update":                  awx_project_update.Icon,
		"awx/schedule_create":                 awx_schedule_create.Icon,
		"awx/schedule_delete":                 awx_schedule_delete.Icon,
		"awx/schedule_list":                   awx_schedule_list.Icon,
		"awx/schedule_update":                 awx_schedule_update.Icon,
		"awx/team_list":                       awx_team_list.Icon,
		"awx/user_list":                       awx_user_list.Icon,
		"awx/workflow_job_get":                awx_workflow_job_get.Icon,
		"awx/workflow_job_nodes_list":         awx_workflow_job_nodes_list.Icon,
		"awx/workflow_job_relaunch":           awx_workflow_job_relaunch.Icon,
		"awx/workflow_launch":                 awx_workflow_launch.Icon,
		"awx/workflow_template_get":           awx_workflow_template_get.Icon,
		"awx/workflow_template_list":          awx_workflow_template_list.Icon,
	}
}

// awxDestructiveActions are the AWX actions that permanently change state and must
// carry the confirm_destructive guard. Two entries are worth explaining, because
// both look wrong until you read the action:
//
//   - project_update is here even though it is a PATCH, not a delete. Pointing an
//     existing project at a different repository or branch silently changes what
//     EVERY job template using that project runs. It is a shared, live resource.
//
//   - adhoc_command_run is here because it is the most dangerous action in the node:
//     it can run `shell: rm -rf /` across an entire inventory.
//
// And one deliberate ABSENCE: host_group_assign disassociates via /groups/{id}/hosts/,
// which breaks the membership and leaves the host alive. (Doing the same through the
// INVENTORY sublist would hard-delete the host, because that relation carries a
// parent_key — which is exactly why the action does not use that route.) It is not
// destructive, and the test below asserts it carries no guard.
func awxDestructiveActions() map[string]bool {
	return map[string]bool{
		"awx/adhoc_command_run": true,
		"awx/credential_delete": true,
		"awx/group_delete":      true,
		"awx/host_delete":       true,
		"awx/inventory_delete":  true,
		"awx/job_cancel":        true,
		"awx/project_delete":    true,
		"awx/project_update":    true,
		"awx/schedule_delete":   true,
	}
}

// awxBadges are the badge glyphs the AWX icons use, on top of the ones the
// Kubernetes and Helm actions already needed. Every name here was checked against
// editor/app/components/icons/paths.ts — a badge that is missing there renders as a
// silent "?" in the palette, which no compiler and no other test would catch.
//
// These extend, rather than replace, editorBadges in inputs_drift_test.go: that map
// is a snapshot of the editor's glyph set, so there is exactly one of it.
func awxBadgeIsKnown(badge string) bool {
	return editorBadges[badge]
}

// registerAWXCoverage tells TestEveryRegisteredActionIsCovered (in
// inputs_drift_test.go) that these 59 actions are enforced HERE. Without it, that
// test fails every AWX action as uncovered — which is the seam working as intended:
// an action must be named by SOME table in this package.
func init() {
	for id := range awxActionInputs() {
		coveredElsewhere[id] = true
	}
}

// TestAWXAuthBlockDoesNotDrift is the whole reason this file exists.
//
// Every action's first seven inputs must reproduce awx.AuthInputs exactly — name,
// type, label, placeholder, required, options and visible_when, in order. The
// visible_when conditions are the sharp edge: api_token is shown when auth_method is
// "" or "token" (the empty string matters — it is what an unset dropdown reads as, so
// a fresh node shows the token field), and awx_username / awx_password only when it
// is "basic". A copy that drops the "" would hide the token field on a brand-new node
// and the operator would see a credential form with nothing to fill in.
func TestAWXAuthBlockDoesNotDrift(t *testing.T) {
	want := awx.AuthInputs

	for id, inputs := range awxActionInputs() {
		if len(inputs) < len(want) {
			t.Errorf("%s: has %d inputs, fewer than the %d-field credential block", id, len(inputs), len(want))
			continue
		}
		for i := range want {
			if !reflect.DeepEqual(inputs[i], want[i]) {
				t.Errorf("%s: credential input %d (%q) has drifted from awx.AuthInputs\n got: %+v\nwant: %+v",
					id, i, want[i].Name, inputs[i], want[i])
			}
		}
	}
}

// TestAWXNoResourceInputShadowsACredential guards the input-name collision that
// core.FindConnection makes possible: it returns the FIRST input whose name matches,
// and the credential block is declared first — so a resource field that reuses a
// credential's name silently reads the CREDENTIAL instead, and the action quietly
// operates on the wrong value.
//
// AWX is unusually exposed to this. url, token, username, password and their kin are
// all plausible names on both sides of the form, which is why the credential block
// spells them awx_url, api_token, awx_username, awx_password rather than the obvious
// thing. This test is what stops action number 60 from taking the obvious thing.
func TestAWXNoResourceInputShadowsACredential(t *testing.T) {
	credential := map[string]bool{}
	for _, c := range awx.AuthInputs {
		credential[c.Name] = true
	}

	for id, inputs := range awxActionInputs() {
		if len(inputs) < len(awx.AuthInputs) {
			continue // TestAWXAuthBlockDoesNotDrift already reports this
		}
		for _, c := range inputs[len(awx.AuthInputs):] {
			if credential[c.Name] {
				t.Errorf("%s: resource input %q shadows the credential input of the same name — "+
					"core.FindConnection would return the credential; rename it (e.g. the credential block uses awx_url, not url)",
					id, c.Name)
			}
		}
	}
}

// TestAWXDestructiveActionsAreGuarded pins the confirm_destructive contract in both
// directions: a destructive action carries the guard LAST and Required, and an action
// that carries the guard is one we have declared destructive.
//
// It is a boolean rather than a typed confirmation string so that the editor renders a
// checkbox — but the editor also offers a variable picker on booleans, which is the
// point: a flow can bind it to ${var.approved} coming out of a Human-in-the-Loop node
// and gate the delete on a decision made at RUN time, not on a box someone ticked once
// at design time. An AI tool loop cannot tick a box it was never handed.
func TestAWXDestructiveActionsAreGuarded(t *testing.T) {
	destructive := awxDestructiveActions()

	for id, inputs := range awxActionInputs() {
		if len(inputs) == 0 {
			t.Errorf("%s: has no inputs at all", id)
			continue
		}
		last := inputs[len(inputs)-1]
		guarded := last.Name == "confirm_destructive"

		if destructive[id] && !guarded {
			t.Errorf("%s: is destructive, but its LAST input is %q, not confirm_destructive — "+
				"the guard must be last so it reads as the final word in the form", id, last.Name)
		}
		if !destructive[id] && guarded {
			t.Errorf("%s: carries confirm_destructive but is not listed in awxDestructiveActions() — "+
				"either it is destructive (list it) or the guard is spurious (drop it)", id)
		}
		if guarded {
			if !last.Required {
				t.Errorf("%s: confirm_destructive must be Required, or the operator can leave it unset and the guard is decorative", id)
			}
			if last.Type != core.ConnectionTypeBoolean {
				t.Errorf("%s: confirm_destructive must be a boolean (so it can be bound to a variable), got %q", id, last.Type)
			}
		}
	}

	// A destructive action that has been DELETED would otherwise leave a silent
	// entry in the table above, and the guard would look enforced when nothing is.
	inputs := awxActionInputs()
	for id := range destructive {
		if _, ok := inputs[id]; !ok {
			t.Errorf("%s is listed as destructive but is not in awxActionInputs() — was it renamed or removed?", id)
		}
	}
}

// TestAWXIconsResolve keeps every icon inside the glyph set the editor actually
// ships. An unknown base or badge compiles cleanly, passes every other test, and
// renders as a "?" in the node palette — a defect nothing else in the build can see.
func TestAWXIconsResolve(t *testing.T) {
	for id, icon := range awxActionIcons() {
		base, badge, composed := strings.Cut(icon, "+")
		if !composed {
			t.Errorf("%s: icon %q is not a base+badge composition", id, icon)
			continue
		}
		if base != "ansible" {
			t.Errorf("%s: icon base is %q, not \"ansible\" — every action in this node wears the Ansible mark", id, base)
		}
		if !awxBadgeIsKnown(badge) {
			t.Errorf("%s: icon badge %q is not in editor/app/components/icons/paths.ts — it would render as \"?\" in the palette", id, badge)
		}
	}
}

// TestAWXStandardOutputsPresent pins the three outputs the platform depends on:
// success drives the soft-failure path (an action that fails a call still returns
// outputs, with success=false, so the flow can branch on it rather than dying),
// error carries the message, and tool_result is what the AI tool loop shows the model.
//
// The sibling file has a test of this name too, but it ranges over ITS tables — the
// AWX actions would be checked by nothing without this.
func TestAWXStandardOutputsPresent(t *testing.T) {
	for id, outputs := range awxActionOutputs() {
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

// TestAWXTableCoversEveryActionOnDisk is the belt to TestEveryRegisteredActionIsCovered's
// braces. That test proves the table matches what is REGISTERED; this one proves the
// count matches what was DESIGNED. If action 60 lands and nobody adds it here, the
// count is what says so.
func TestAWXTableCoversEveryActionOnDisk(t *testing.T) {
	const designed = 59
	if got := len(awxActionInputs()); got != designed {
		t.Errorf("awxActionInputs() covers %d actions, expected %d — a new AWX action must be added to the tables in this file", got, designed)
	}
}

// TestAWXDeleteActionsAreDeclaredDestructive closes the hole the table above cannot
// see on its own.
//
// awxDestructiveActions() is hand-maintained, so the guard test only checks the
// actions someone REMEMBERED to list. An action that deletes something, is left out
// of the table AND ships without a guard passes every assertion above — the two
// mistakes cancel out. This asserts the property structurally instead: anything named
// *_delete is destructive, full stop, whatever the table says.
func TestAWXDeleteActionsAreDeclaredDestructive(t *testing.T) {
	destructive := awxDestructiveActions()

	for id := range awxActionInputs() {
		if !strings.HasSuffix(id, "_delete") {
			continue
		}
		if !destructive[id] {
			t.Errorf("%s deletes an AWX resource but is not in awxDestructiveActions() — "+
				"add it there and give it a confirm_destructive guard", id)
		}
	}
}

// TestAWXTriggerIconResolves pins the one property the trigger shares with the 59
// actions: it wears the same Ansible mark, so the AAP / AWX group reads as one node
// in the palette rather than as a stray webhook that happens to be named AWX.
//
// Unlike the actions it is a BARE base with no badge — triggers are not composed —
// so it is checked here rather than folded into TestAWXIconsResolve, whose whole
// contract is base+badge.
func TestAWXTriggerIconResolves(t *testing.T) {
	if awx_trigger.Icon != "ansible" {
		t.Errorf("trigger/awx_webhook: icon is %q, want \"ansible\" — the trigger must wear the same mark as the 59 actions", awx_trigger.Icon)
	}
	if strings.Contains(awx_trigger.Icon, "+") {
		t.Errorf("trigger/awx_webhook: icon %q is composed; triggers use a bare base", awx_trigger.Icon)
	}
	if awx_trigger.Type != core.ActionTypeTrigger {
		t.Errorf("trigger/awx_webhook: Type is %d, want ActionTypeTrigger (%d) — a trigger that registers as an action never gets a webhook URL",
			awx_trigger.Type, core.ActionTypeTrigger)
	}
}
