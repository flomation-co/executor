// Cross-action invariants for the Azure ▸ Entra ID node, on the precedent of
// actions/infrastructure/awx_inputs_drift_test.go.
//
// What this file is really for: all 25 Entra actions re-declare the credential
// block INLINE, because the manifest generator AST-parses the Inputs literal
// and cannot see through a package-level variable. entra.AuthInputs is
// therefore documentation, not enforcement — 25 copies of four fields, free to
// drift one paste at a time. This is the enforcement. A copy that drifts fails
// CI with the action and the field named.
package entra_test

import (
	"reflect"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	entra "flomation.app/automate/executor/actions/azure/entra"

	entra_deleted_item_restore "flomation.app/automate/executor/actions/azure/entra/deleted_item_restore"
	entra_group_add_members "flomation.app/automate/executor/actions/azure/entra/group_add_members"
	entra_group_create "flomation.app/automate/executor/actions/azure/entra/group_create"
	entra_group_delete "flomation.app/automate/executor/actions/azure/entra/group_delete"
	entra_group_get "flomation.app/automate/executor/actions/azure/entra/group_get"
	entra_group_get_all "flomation.app/automate/executor/actions/azure/entra/group_get_all"
	entra_group_list_members "flomation.app/automate/executor/actions/azure/entra/group_list_members"
	entra_group_list_owners "flomation.app/automate/executor/actions/azure/entra/group_list_owners"
	entra_group_remove_member "flomation.app/automate/executor/actions/azure/entra/group_remove_member"
	entra_group_update "flomation.app/automate/executor/actions/azure/entra/group_update"
	entra_guest_invite "flomation.app/automate/executor/actions/azure/entra/guest_invite"
	entra_subscribed_skus_get_all "flomation.app/automate/executor/actions/azure/entra/subscribed_skus_get_all"
	entra_user_add_to_group "flomation.app/automate/executor/actions/azure/entra/user_add_to_group"
	entra_user_assign_license "flomation.app/automate/executor/actions/azure/entra/user_assign_license"
	entra_user_check_group_membership "flomation.app/automate/executor/actions/azure/entra/user_check_group_membership"
	entra_user_create "flomation.app/automate/executor/actions/azure/entra/user_create"
	entra_user_delete "flomation.app/automate/executor/actions/azure/entra/user_delete"
	entra_user_get "flomation.app/automate/executor/actions/azure/entra/user_get"
	entra_user_get_all "flomation.app/automate/executor/actions/azure/entra/user_get_all"
	entra_user_get_manager "flomation.app/automate/executor/actions/azure/entra/user_get_manager"
	entra_user_list_groups "flomation.app/automate/executor/actions/azure/entra/user_list_groups"
	entra_user_remove_from_group "flomation.app/automate/executor/actions/azure/entra/user_remove_from_group"
	entra_user_revoke_sessions "flomation.app/automate/executor/actions/azure/entra/user_revoke_sessions"
	entra_user_set_manager "flomation.app/automate/executor/actions/azure/entra/user_set_manager"
	entra_user_update "flomation.app/automate/executor/actions/azure/entra/user_update"
)

// entraActionInputs is the table every assertion below ranges over. All 25 actions.
func entraActionInputs() map[string][]core.Connection {
	return map[string][]core.Connection{
		"entra/deleted_item_restore":        entra_deleted_item_restore.Inputs[:],
		"entra/group_add_members":           entra_group_add_members.Inputs[:],
		"entra/group_create":                entra_group_create.Inputs[:],
		"entra/group_delete":                entra_group_delete.Inputs[:],
		"entra/group_get":                   entra_group_get.Inputs[:],
		"entra/group_get_all":               entra_group_get_all.Inputs[:],
		"entra/group_list_members":          entra_group_list_members.Inputs[:],
		"entra/group_list_owners":           entra_group_list_owners.Inputs[:],
		"entra/group_remove_member":         entra_group_remove_member.Inputs[:],
		"entra/group_update":                entra_group_update.Inputs[:],
		"entra/guest_invite":                entra_guest_invite.Inputs[:],
		"entra/subscribed_skus_get_all":     entra_subscribed_skus_get_all.Inputs[:],
		"entra/user_add_to_group":           entra_user_add_to_group.Inputs[:],
		"entra/user_assign_license":         entra_user_assign_license.Inputs[:],
		"entra/user_check_group_membership": entra_user_check_group_membership.Inputs[:],
		"entra/user_create":                 entra_user_create.Inputs[:],
		"entra/user_delete":                 entra_user_delete.Inputs[:],
		"entra/user_get":                    entra_user_get.Inputs[:],
		"entra/user_get_all":                entra_user_get_all.Inputs[:],
		"entra/user_get_manager":            entra_user_get_manager.Inputs[:],
		"entra/user_list_groups":            entra_user_list_groups.Inputs[:],
		"entra/user_remove_from_group":      entra_user_remove_from_group.Inputs[:],
		"entra/user_revoke_sessions":        entra_user_revoke_sessions.Inputs[:],
		"entra/user_set_manager":            entra_user_set_manager.Inputs[:],
		"entra/user_update":                 entra_user_update.Inputs[:],
	}
}

// entraActionOutputs backs the standard-outputs assertion.
func entraActionOutputs() map[string][]core.Connection {
	return map[string][]core.Connection{
		"entra/deleted_item_restore":        entra_deleted_item_restore.Outputs[:],
		"entra/group_add_members":           entra_group_add_members.Outputs[:],
		"entra/group_create":                entra_group_create.Outputs[:],
		"entra/group_delete":                entra_group_delete.Outputs[:],
		"entra/group_get":                   entra_group_get.Outputs[:],
		"entra/group_get_all":               entra_group_get_all.Outputs[:],
		"entra/group_list_members":          entra_group_list_members.Outputs[:],
		"entra/group_list_owners":           entra_group_list_owners.Outputs[:],
		"entra/group_remove_member":         entra_group_remove_member.Outputs[:],
		"entra/group_update":                entra_group_update.Outputs[:],
		"entra/guest_invite":                entra_guest_invite.Outputs[:],
		"entra/subscribed_skus_get_all":     entra_subscribed_skus_get_all.Outputs[:],
		"entra/user_add_to_group":           entra_user_add_to_group.Outputs[:],
		"entra/user_assign_license":         entra_user_assign_license.Outputs[:],
		"entra/user_check_group_membership": entra_user_check_group_membership.Outputs[:],
		"entra/user_create":                 entra_user_create.Outputs[:],
		"entra/user_delete":                 entra_user_delete.Outputs[:],
		"entra/user_get":                    entra_user_get.Outputs[:],
		"entra/user_get_all":                entra_user_get_all.Outputs[:],
		"entra/user_get_manager":            entra_user_get_manager.Outputs[:],
		"entra/user_list_groups":            entra_user_list_groups.Outputs[:],
		"entra/user_remove_from_group":      entra_user_remove_from_group.Outputs[:],
		"entra/user_revoke_sessions":        entra_user_revoke_sessions.Outputs[:],
		"entra/user_set_manager":            entra_user_set_manager.Outputs[:],
		"entra/user_update":                 entra_user_update.Outputs[:],
	}
}

// entraActionIcons backs the icon-resolution assertion.
func entraActionIcons() map[string]string {
	return map[string]string{
		"entra/deleted_item_restore":        entra_deleted_item_restore.Icon,
		"entra/group_add_members":           entra_group_add_members.Icon,
		"entra/group_create":                entra_group_create.Icon,
		"entra/group_delete":                entra_group_delete.Icon,
		"entra/group_get":                   entra_group_get.Icon,
		"entra/group_get_all":               entra_group_get_all.Icon,
		"entra/group_list_members":          entra_group_list_members.Icon,
		"entra/group_list_owners":           entra_group_list_owners.Icon,
		"entra/group_remove_member":         entra_group_remove_member.Icon,
		"entra/group_update":                entra_group_update.Icon,
		"entra/guest_invite":                entra_guest_invite.Icon,
		"entra/subscribed_skus_get_all":     entra_subscribed_skus_get_all.Icon,
		"entra/user_add_to_group":           entra_user_add_to_group.Icon,
		"entra/user_assign_license":         entra_user_assign_license.Icon,
		"entra/user_check_group_membership": entra_user_check_group_membership.Icon,
		"entra/user_create":                 entra_user_create.Icon,
		"entra/user_delete":                 entra_user_delete.Icon,
		"entra/user_get":                    entra_user_get.Icon,
		"entra/user_get_all":                entra_user_get_all.Icon,
		"entra/user_get_manager":            entra_user_get_manager.Icon,
		"entra/user_list_groups":            entra_user_list_groups.Icon,
		"entra/user_remove_from_group":      entra_user_remove_from_group.Icon,
		"entra/user_revoke_sessions":        entra_user_revoke_sessions.Icon,
		"entra/user_set_manager":            entra_user_set_manager.Icon,
		"entra/user_update":                 entra_user_update.Icon,
	}
}

// entraListActions are the get-many actions, which swap id/result for
// results/count in the standard output contract.
func entraListActions() map[string]bool {
	return map[string]bool{
		"entra/group_get_all":           true,
		"entra/group_list_members":      true,
		"entra/group_list_owners":       true,
		"entra/subscribed_skus_get_all": true,
		"entra/user_get_all":            true,
		"entra/user_list_groups":        true,
	}
}

// entraBadges is the badge glyphs the Entra icons compose onto the azure base.
// Every name here was checked against editor/app/components/icons/paths.ts —
// a badge missing there renders as a silent "?" in the palette, which no
// compiler and no other test would catch.
var entraBadges = map[string]bool{
	"ban":            true,
	"check":          true,
	"clipboard-list": true,
	"envelope":       true,
	"key":            true,
	"list":           true,
	"pen":            true,
	"people-group":   true,
	"plus":           true,
	"rotate-left":    true,
	"trash":          true,
	"user":           true,
	"user-group":     true,
	"user-minus":     true,
	"user-plus":      true,
}

// TestEntraAuthBlockDoesNotDrift is the whole reason this file exists.
//
// Every action's first four inputs must reproduce entra.AuthInputs exactly —
// name, type, label, placeholder and required, in order. The names are the
// sharp edge: azure_tenant_id / azure_client_id / azure_client_secret /
// graph_endpoint are AST-parsed by the manifest generator and mirrored by the
// api's dynamic-options params lists, so one drifted copy breaks the live
// dropdowns for that action only — the kind of defect nobody notices until a
// customer does.
func TestEntraAuthBlockDoesNotDrift(t *testing.T) {
	want := entra.AuthInputs

	for id, inputs := range entraActionInputs() {
		if len(inputs) < len(want) {
			t.Errorf("%s: has %d inputs, fewer than the %d-field credential block", id, len(inputs), len(want))
			continue
		}
		for i := range want {
			if !reflect.DeepEqual(inputs[i], want[i]) {
				t.Errorf("%s: credential input %d (%q) has drifted from entra.AuthInputs\n got: %+v\nwant: %+v",
					id, i, want[i].Name, inputs[i], want[i])
			}
		}
	}
}

// TestEntraNoResourceInputShadowsACredential guards the collision that
// core.FindConnection makes possible: it returns the FIRST input whose name
// matches, and the credential block is declared first — so a resource field
// that reuses a credential's name silently reads the CREDENTIAL instead.
func TestEntraNoResourceInputShadowsACredential(t *testing.T) {
	credential := map[string]bool{}
	for _, c := range entra.AuthInputs {
		credential[c.Name] = true
	}

	for id, inputs := range entraActionInputs() {
		if len(inputs) < len(entra.AuthInputs) {
			continue // TestEntraAuthBlockDoesNotDrift already reports this
		}
		for _, c := range inputs[len(entra.AuthInputs):] {
			if credential[c.Name] {
				t.Errorf("%s: resource input %q shadows the credential input of the same name — "+
					"core.FindConnection would return the credential; rename it", id, c.Name)
			}
		}
	}
}

// TestEntraIconsResolve keeps every icon inside the glyph set the editor
// actually ships. An unknown base or badge compiles cleanly, passes every
// other test, and renders as a "?" in the node palette.
func TestEntraIconsResolve(t *testing.T) {
	for id, icon := range entraActionIcons() {
		base, badge, composed := strings.Cut(icon, "+")
		if !composed {
			t.Errorf("%s: icon %q is not a base+badge composition", id, icon)
			continue
		}
		if base != "azure" {
			t.Errorf("%s: icon base is %q, not \"azure\" — every action in this node wears the Azure mark", id, base)
		}
		if !entraBadges[badge] {
			t.Errorf("%s: icon badge %q is not in the verified badge set (checked against editor paths.ts) — it would render as \"?\" in the palette", id, badge)
		}
	}
}

// TestEntraStandardOutputsPresent pins the outputs the platform depends on:
// success drives the soft-failure path, error carries the message, tool_result
// is what the AI tool loop shows the model — plus the id/result vs
// results/count split between single-object and list actions.
func TestEntraStandardOutputsPresent(t *testing.T) {
	listActions := entraListActions()

	for id, outputs := range entraActionOutputs() {
		have := map[string]bool{}
		for _, o := range outputs {
			have[o.Name] = true
		}
		for _, required := range []string{"success", "error", "tool_result"} {
			if !have[required] {
				t.Errorf("%s: missing the %q output", id, required)
			}
		}
		if listActions[id] {
			for _, required := range []string{"results", "count"} {
				if !have[required] {
					t.Errorf("%s: is a list action but missing the %q output", id, required)
				}
			}
		} else {
			for _, required := range []string{"id", "result"} {
				if !have[required] {
					t.Errorf("%s: missing the %q output", id, required)
				}
			}
		}
	}
}

// TestEntraTableCoversEveryActionOnDisk pins the designed count. If action 26
// lands and nobody adds it to the tables in this file, this is what says so.
func TestEntraTableCoversEveryActionOnDisk(t *testing.T) {
	const designed = 25
	if got := len(entraActionInputs()); got != designed {
		t.Errorf("entraActionInputs() covers %d actions, expected %d — a new Entra action must be added to the tables in this file", got, designed)
	}
	if got := len(entraActionOutputs()); got != designed {
		t.Errorf("entraActionOutputs() covers %d actions, expected %d", got, designed)
	}
	if got := len(entraActionIcons()); got != designed {
		t.Errorf("entraActionIcons() covers %d actions, expected %d", got, designed)
	}
}

// TestEntraListControlsUseConsistentLabels pins the paging-control convention.
// A non-technical operator learns Return All / Limit once and must recognise
// them on every get-many action.
func TestEntraListControlsUseConsistentLabels(t *testing.T) {
	canonical := map[string]string{
		"return_all": "Return All (follow every page)",
		"limit":      "Limit",
	}
	for id, inputs := range entraActionInputs() {
		for _, in := range inputs {
			if want, ok := canonical[in.Name]; ok && in.Label != want {
				t.Errorf("%s: %q Label = %q, want %q — the paging controls must read the same across every list action",
					id, in.Name, in.Label, want)
			}
		}
	}
}
