// Cross-action invariants for the Azure ▸ Table Storage node.
//
// All 14 azure/tables actions re-declare the nine-field credential block
// INLINE, because the manifest generator AST-parses the Inputs literal and
// cannot see through a package-level variable. tables.AuthInputs is therefore
// documentation, not enforcement — 14 copies of nine fields, free to drift one
// paste at a time. This file is the enforcement: a copy that drifts fails CI
// with the action and the field named. Modelled on
// actions/azure/storage/storage_inputs_drift_test.go.
package tables_test

import (
	"reflect"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	tables "flomation.app/automate/executor/actions/azure/tables"

	entity_batch "flomation.app/automate/executor/actions/azure/tables/entity_batch"
	entity_delete "flomation.app/automate/executor/actions/azure/tables/entity_delete"
	entity_get "flomation.app/automate/executor/actions/azure/tables/entity_get"
	entity_insert "flomation.app/automate/executor/actions/azure/tables/entity_insert"
	entity_query "flomation.app/automate/executor/actions/azure/tables/entity_query"
	entity_update "flomation.app/automate/executor/actions/azure/tables/entity_update"
	entity_upsert "flomation.app/automate/executor/actions/azure/tables/entity_upsert"
	service_get_properties "flomation.app/automate/executor/actions/azure/tables/service_get_properties"
	table_create "flomation.app/automate/executor/actions/azure/tables/table_create"
	table_delete "flomation.app/automate/executor/actions/azure/tables/table_delete"
	table_generate_sas "flomation.app/automate/executor/actions/azure/tables/table_generate_sas"
	table_get_access_policy "flomation.app/automate/executor/actions/azure/tables/table_get_access_policy"
	table_get_all "flomation.app/automate/executor/actions/azure/tables/table_get_all"
	table_set_access_policy "flomation.app/automate/executor/actions/azure/tables/table_set_access_policy"
)

// tablesActionInputs is the table every assertion below ranges over. All 14
// actions.
func tablesActionInputs() map[string][]core.Connection {
	return map[string][]core.Connection{
		"azure/tables/entity_batch":            entity_batch.Inputs[:],
		"azure/tables/entity_delete":           entity_delete.Inputs[:],
		"azure/tables/entity_get":              entity_get.Inputs[:],
		"azure/tables/entity_insert":           entity_insert.Inputs[:],
		"azure/tables/entity_query":            entity_query.Inputs[:],
		"azure/tables/entity_update":           entity_update.Inputs[:],
		"azure/tables/entity_upsert":           entity_upsert.Inputs[:],
		"azure/tables/service_get_properties":  service_get_properties.Inputs[:],
		"azure/tables/table_create":            table_create.Inputs[:],
		"azure/tables/table_delete":            table_delete.Inputs[:],
		"azure/tables/table_generate_sas":      table_generate_sas.Inputs[:],
		"azure/tables/table_get_access_policy": table_get_access_policy.Inputs[:],
		"azure/tables/table_get_all":           table_get_all.Inputs[:],
		"azure/tables/table_set_access_policy": table_set_access_policy.Inputs[:],
	}
}

// tablesActionOutputs backs the standard-outputs assertion.
func tablesActionOutputs() map[string][]core.Connection {
	return map[string][]core.Connection{
		"azure/tables/entity_batch":            entity_batch.Outputs[:],
		"azure/tables/entity_delete":           entity_delete.Outputs[:],
		"azure/tables/entity_get":              entity_get.Outputs[:],
		"azure/tables/entity_insert":           entity_insert.Outputs[:],
		"azure/tables/entity_query":            entity_query.Outputs[:],
		"azure/tables/entity_update":           entity_update.Outputs[:],
		"azure/tables/entity_upsert":           entity_upsert.Outputs[:],
		"azure/tables/service_get_properties":  service_get_properties.Outputs[:],
		"azure/tables/table_create":            table_create.Outputs[:],
		"azure/tables/table_delete":            table_delete.Outputs[:],
		"azure/tables/table_generate_sas":      table_generate_sas.Outputs[:],
		"azure/tables/table_get_access_policy": table_get_access_policy.Outputs[:],
		"azure/tables/table_get_all":           table_get_all.Outputs[:],
		"azure/tables/table_set_access_policy": table_set_access_policy.Outputs[:],
	}
}

// tablesActionIcons backs the icon-resolution assertion.
func tablesActionIcons() map[string]string {
	return map[string]string{
		"azure/tables/entity_batch":            entity_batch.Icon,
		"azure/tables/entity_delete":           entity_delete.Icon,
		"azure/tables/entity_get":              entity_get.Icon,
		"azure/tables/entity_insert":           entity_insert.Icon,
		"azure/tables/entity_query":            entity_query.Icon,
		"azure/tables/entity_update":           entity_update.Icon,
		"azure/tables/entity_upsert":           entity_upsert.Icon,
		"azure/tables/service_get_properties":  service_get_properties.Icon,
		"azure/tables/table_create":            table_create.Icon,
		"azure/tables/table_delete":            table_delete.Icon,
		"azure/tables/table_generate_sas":      table_generate_sas.Icon,
		"azure/tables/table_get_access_policy": table_get_access_policy.Icon,
		"azure/tables/table_get_all":           table_get_all.Icon,
		"azure/tables/table_set_access_policy": table_set_access_policy.Icon,
	}
}

// tablesBadges is the badge glyph set these icons draw on. Every name here was
// checked against editor/app/components/icons/paths.ts — a badge missing there
// renders as a silent "?" in the palette, which no compiler and no other test
// would catch. (table-list and table-cells do NOT exist there, which is why
// the sub-category wears plain "table".)
var tablesBadges = map[string]bool{
	"gear":             true,
	"key":              true,
	"layer-group":      true,
	"list":             true,
	"lock":             true,
	"magnifying-glass": true,
	"pen":              true,
	"plus":             true,
	"trash":            true,
}

// listActions use results/count instead of id/result.
var listActions = map[string]bool{
	"azure/tables/entity_query":            true,
	"azure/tables/table_get_all":           true,
	"azure/tables/table_get_access_policy": true,
}

// TestTablesAuthBlockDoesNotDrift is the whole reason this file exists.
//
// Every action's first nine inputs must reproduce tables.AuthInputs exactly —
// name, type, label, placeholder, required, options and visible_when, in order.
//
// The visible_when values are the sharp edge. account_key shows when
// auth_method is "" or "shared_key": the empty string matters, because it is
// what an untouched dropdown reads as, so a fresh node must still show the key
// field. account_name hides under connection_string (the string carries the
// account itself) and the azure_* fields show only under entra. A copy that
// drops the "" would hide the key field on a brand-new node — the sort of
// break that looks like the editor is broken rather than the action.
func TestTablesAuthBlockDoesNotDrift(t *testing.T) {
	want := tables.AuthInputs

	for id, inputs := range tablesActionInputs() {
		if len(inputs) < len(want) {
			t.Errorf("%s: has %d inputs, fewer than the %d-field credential block", id, len(inputs), len(want))
			continue
		}
		for i := range want {
			if !reflect.DeepEqual(inputs[i], want[i]) {
				t.Errorf("%s: credential input %d (%q) has drifted from tables.AuthInputs\n got: %+v\nwant: %+v",
					id, i, want[i].Name, inputs[i], want[i])
			}
		}
	}
}

// TestTablesNoResourceInputShadowsACredential guards the input-name collision
// core.FindConnection makes possible: it returns the FIRST input whose name
// matches, and the credential block is declared first — so a resource field
// that reuses a credential's name silently reads the CREDENTIAL instead.
//
// "table" is the live risk here: it is one careless rename away from
// "account_name", and a Tables node is full of fields that want to be called
// something short.
func TestTablesNoResourceInputShadowsACredential(t *testing.T) {
	credential := map[string]bool{}
	for _, c := range tables.AuthInputs {
		credential[c.Name] = true
	}

	for id, inputs := range tablesActionInputs() {
		if len(inputs) < len(tables.AuthInputs) {
			continue // TestTablesAuthBlockDoesNotDrift already reports this
		}
		for _, c := range inputs[len(tables.AuthInputs):] {
			if credential[c.Name] {
				t.Errorf("%s: resource input %q shadows the credential input of the same name — "+
					"core.FindConnection would return the credential; rename it", id, c.Name)
			}
		}
	}
}

// updateModeActions is every action that writes a whole row and must therefore
// let the operator choose between merge and replace.
//
// The set is deliberately CLOSED and the test asserts both directions. The
// absences are the interesting half: entity_insert creates a row, so there is
// nothing to merge with or replace; entity_delete removes one; and
// entity_batch carries the mode PER CHANGE inside its actions array, because a
// transaction may legitimately mix merges and replaces.
var updateModeActions = map[string]bool{
	"azure/tables/entity_upsert": true,
	"azure/tables/entity_update": true,
}

// TestTablesUpdateModeDoesNotDrift pins the merge/replace dropdown wherever it
// appears, and pins that it appears nowhere else.
//
// The option LABELS are the substance, not decoration. Replace deletes every
// property the supplied row does not mention — silently, with no warning —
// which is the single easiest way to lose data in this node. A copy that
// re-words them back to a bare "Merge"/"Replace" would be a regression, and
// one nobody would notice until an operator had already lost a column.
func TestTablesUpdateModeDoesNotDrift(t *testing.T) {
	for id, inputs := range tablesActionInputs() {
		var found *core.Connection
		for i := range inputs {
			if inputs[i].Name == "update_mode" {
				found = &inputs[i]
			}
		}
		switch {
		case updateModeActions[id] && found == nil:
			t.Errorf("%s: writes a whole row but offers no update_mode — the operator cannot choose between merge and replace", id)
		case !updateModeActions[id] && found != nil:
			t.Errorf("%s: declares an update_mode input, but this action does not take one — the field would do nothing", id)
		case found != nil && !reflect.DeepEqual(*found, tables.UpdateModeInput):
			t.Errorf("%s: update_mode has drifted from tables.UpdateModeInput\n got: %+v\nwant: %+v", id, *found, tables.UpdateModeInput)
		}
	}
}

// TestTablesUpdateModeDefaultsToMerge pins the default that keeps data alive.
// An unset dropdown reads as "", and "" must mean merge: if blank ever meant
// replace, every operator who did not touch the dropdown would silently delete
// the columns they did not mention.
func TestTablesUpdateModeDefaultsToMerge(t *testing.T) {
	for _, o := range tables.UpdateModeInput.Options {
		if o.Value == "" {
			t.Fatalf("update_mode offers an explicit blank option (%q) — blank must be reachable only as the untouched default", o.Name)
		}
	}
	if tables.UpdateModeInput.Options[0].Value != "merge" {
		t.Errorf("the first update_mode option is %q — merge must lead, it is the safe one",
			tables.UpdateModeInput.Options[0].Value)
	}
	if tables.UpdateModeInput.Required {
		t.Error("update_mode must not be Required — blank is a valid state meaning merge")
	}
}

// TestTablesETagIsNotACredential pins where the concurrency field lives. It is
// an operator-supplied fact about one call, not a credential: it must sit
// AFTER the nine-field auth block, or the auth-block drift assertion — and the
// api's dynamic-options params, which name those nine inputs positionally by
// name — would both be reading a different shape than they were written
// against.
func TestTablesETagIsNotACredential(t *testing.T) {
	for _, c := range tables.AuthInputs {
		if c.Name == "etag" {
			t.Fatal("etag has been added to tables.AuthInputs — it is a resource field, not a credential")
		}
	}
	for id, inputs := range tablesActionInputs() {
		for i, c := range inputs {
			if c.Name == "etag" && i < len(tables.AuthInputs) {
				t.Errorf("%s: etag is at index %d, inside the %d-field credential block", id, i, len(tables.AuthInputs))
			}
		}
	}
}

// TestTablesPointOpsRequireBothKeys pins the composite identity. PartitionKey
// and RowKey together ARE the row's identity — one without the other addresses
// nothing — so any action offering one must require both, and must say so.
func TestTablesPointOpsRequireBothKeys(t *testing.T) {
	for id, inputs := range tablesActionInputs() {
		var partition, row *core.Connection
		for i := range inputs {
			switch inputs[i].Name {
			case "partition_key":
				partition = &inputs[i]
			case "row_key":
				row = &inputs[i]
			}
		}
		if partition == nil && row == nil {
			continue
		}
		if partition == nil || row == nil {
			t.Errorf("%s: declares only one half of the composite identity — a PartitionKey without a RowKey (or vice versa) addresses no row", id)
			continue
		}
		if !partition.Required || !row.Required {
			t.Errorf("%s: partition_key and row_key must both be Required — together they are the row's identity", id)
		}
		// The placeholders carry the data model to an operator who has never
		// used Table Storage. Without them the two fields read as arbitrary
		// jargon, and the operator guesses.
		if !strings.Contains(partition.Placeholder, "group") {
			t.Errorf("%s: the partition_key placeholder must explain what a partition IS, got %q", id, partition.Placeholder)
		}
		if !strings.Contains(row.Placeholder, "identify exactly one row") {
			t.Errorf("%s: the row_key placeholder must explain the composite identity, got %q", id, row.Placeholder)
		}
	}
}

// TestTablesIconsResolve keeps every icon inside the glyph set the editor
// actually ships: the azure base plus a badge from tablesBadges.
func TestTablesIconsResolve(t *testing.T) {
	for id, icon := range tablesActionIcons() {
		base, badge, composed := strings.Cut(icon, "+")
		if !composed {
			t.Errorf("%s: icon %q is not a base+badge composition", id, icon)
			continue
		}
		if base != "azure" {
			t.Errorf("%s: icon base is %q, not \"azure\" — every action in this node wears the Azure mark", id, base)
		}
		if !tablesBadges[badge] {
			t.Errorf("%s: icon badge %q is not in the verified paths.ts glyph set — it would render as \"?\" in the palette", id, badge)
		}
	}
}

// TestTablesStandardOutputsPresent pins the outputs the platform depends on:
// success drives the soft-failure path, error carries the message, tool_result
// is what the AI tool loop shows the model — plus the id/result vs
// results/count baseline split between single-resource and list actions.
func TestTablesStandardOutputsPresent(t *testing.T) {
	for id, outputs := range tablesActionOutputs() {
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
			if !have["results"] || !have["count"] {
				t.Errorf("%s: list actions carry results + count", id)
			}
		} else if !have["id"] || !have["result"] {
			t.Errorf("%s: resource actions carry id + result", id)
		}
	}
}

// TestTablesTableCoversEveryActionOnDisk pins the designed action count. If
// action 15 lands and nobody adds it to the tables here, this is what says so.
func TestTablesTableCoversEveryActionOnDisk(t *testing.T) {
	const designed = 14
	if got := len(tablesActionInputs()); got != designed {
		t.Errorf("tablesActionInputs() covers %d actions, expected %d — a new tables action must be added to the tables in this file", got, designed)
	}
	if got := len(tablesActionOutputs()); got != designed {
		t.Errorf("tablesActionOutputs() covers %d actions, expected %d", got, designed)
	}
	if got := len(tablesActionIcons()); got != designed {
		t.Errorf("tablesActionIcons() covers %d actions, expected %d", got, designed)
	}
}
