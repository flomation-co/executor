// Cross-action invariants for the Azure ▸ Cosmos DB node.
//
// All 18 azure/cosmosdb actions re-declare the eight-field credential block
// INLINE, because the manifest generator AST-parses the Inputs literal and
// cannot see through a package-level variable. cosmosdb.AuthInputs is
// therefore documentation, not enforcement — 18 copies of eight fields, free
// to drift one paste at a time. This file is the enforcement: a copy that
// drifts fails CI with the action and the field named. Modelled on
// actions/infrastructure/awx_inputs_drift_test.go and its azure/storage and
// azure/entra siblings.
package cosmosdb_test

import (
	"reflect"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	cosmosdb "flomation.app/automate/executor/actions/azure/cosmosdb"

	container_create "flomation.app/automate/executor/actions/azure/cosmosdb/container_create"
	container_delete "flomation.app/automate/executor/actions/azure/cosmosdb/container_delete"
	container_get "flomation.app/automate/executor/actions/azure/cosmosdb/container_get"
	container_get_all "flomation.app/automate/executor/actions/azure/cosmosdb/container_get_all"
	container_replace "flomation.app/automate/executor/actions/azure/cosmosdb/container_replace"
	database_create "flomation.app/automate/executor/actions/azure/cosmosdb/database_create"
	database_delete "flomation.app/automate/executor/actions/azure/cosmosdb/database_delete"
	database_get "flomation.app/automate/executor/actions/azure/cosmosdb/database_get"
	database_get_all "flomation.app/automate/executor/actions/azure/cosmosdb/database_get_all"
	item_create "flomation.app/automate/executor/actions/azure/cosmosdb/item_create"
	item_delete "flomation.app/automate/executor/actions/azure/cosmosdb/item_delete"
	item_get "flomation.app/automate/executor/actions/azure/cosmosdb/item_get"
	item_get_all "flomation.app/automate/executor/actions/azure/cosmosdb/item_get_all"
	item_patch "flomation.app/automate/executor/actions/azure/cosmosdb/item_patch"
	item_query "flomation.app/automate/executor/actions/azure/cosmosdb/item_query"
	item_replace "flomation.app/automate/executor/actions/azure/cosmosdb/item_replace"
	throughput_get "flomation.app/automate/executor/actions/azure/cosmosdb/throughput_get"
	throughput_update "flomation.app/automate/executor/actions/azure/cosmosdb/throughput_update"
)

// cosmosActionInputs is the table every assertion below ranges over. All 18
// actions.
func cosmosActionInputs() map[string][]core.Connection {
	return map[string][]core.Connection{
		"azure/cosmosdb/container_create":  container_create.Inputs[:],
		"azure/cosmosdb/container_delete":  container_delete.Inputs[:],
		"azure/cosmosdb/container_get":     container_get.Inputs[:],
		"azure/cosmosdb/container_get_all": container_get_all.Inputs[:],
		"azure/cosmosdb/container_replace": container_replace.Inputs[:],
		"azure/cosmosdb/database_create":   database_create.Inputs[:],
		"azure/cosmosdb/database_delete":   database_delete.Inputs[:],
		"azure/cosmosdb/database_get":      database_get.Inputs[:],
		"azure/cosmosdb/database_get_all":  database_get_all.Inputs[:],
		"azure/cosmosdb/item_create":       item_create.Inputs[:],
		"azure/cosmosdb/item_delete":       item_delete.Inputs[:],
		"azure/cosmosdb/item_get":          item_get.Inputs[:],
		"azure/cosmosdb/item_get_all":      item_get_all.Inputs[:],
		"azure/cosmosdb/item_patch":        item_patch.Inputs[:],
		"azure/cosmosdb/item_query":        item_query.Inputs[:],
		"azure/cosmosdb/item_replace":      item_replace.Inputs[:],
		"azure/cosmosdb/throughput_get":    throughput_get.Inputs[:],
		"azure/cosmosdb/throughput_update": throughput_update.Inputs[:],
	}
}

// cosmosActionOutputs backs the standard-outputs assertion.
func cosmosActionOutputs() map[string][]core.Connection {
	return map[string][]core.Connection{
		"azure/cosmosdb/container_create":  container_create.Outputs[:],
		"azure/cosmosdb/container_delete":  container_delete.Outputs[:],
		"azure/cosmosdb/container_get":     container_get.Outputs[:],
		"azure/cosmosdb/container_get_all": container_get_all.Outputs[:],
		"azure/cosmosdb/container_replace": container_replace.Outputs[:],
		"azure/cosmosdb/database_create":   database_create.Outputs[:],
		"azure/cosmosdb/database_delete":   database_delete.Outputs[:],
		"azure/cosmosdb/database_get":      database_get.Outputs[:],
		"azure/cosmosdb/database_get_all":  database_get_all.Outputs[:],
		"azure/cosmosdb/item_create":       item_create.Outputs[:],
		"azure/cosmosdb/item_delete":       item_delete.Outputs[:],
		"azure/cosmosdb/item_get":          item_get.Outputs[:],
		"azure/cosmosdb/item_get_all":      item_get_all.Outputs[:],
		"azure/cosmosdb/item_patch":        item_patch.Outputs[:],
		"azure/cosmosdb/item_query":        item_query.Outputs[:],
		"azure/cosmosdb/item_replace":      item_replace.Outputs[:],
		"azure/cosmosdb/throughput_get":    throughput_get.Outputs[:],
		"azure/cosmosdb/throughput_update": throughput_update.Outputs[:],
	}
}

// cosmosActionIcons backs the icon-resolution assertion.
func cosmosActionIcons() map[string]string {
	return map[string]string{
		"azure/cosmosdb/container_create":  container_create.Icon,
		"azure/cosmosdb/container_delete":  container_delete.Icon,
		"azure/cosmosdb/container_get":     container_get.Icon,
		"azure/cosmosdb/container_get_all": container_get_all.Icon,
		"azure/cosmosdb/container_replace": container_replace.Icon,
		"azure/cosmosdb/database_create":   database_create.Icon,
		"azure/cosmosdb/database_delete":   database_delete.Icon,
		"azure/cosmosdb/database_get":      database_get.Icon,
		"azure/cosmosdb/database_get_all":  database_get_all.Icon,
		"azure/cosmosdb/item_create":       item_create.Icon,
		"azure/cosmosdb/item_delete":       item_delete.Icon,
		"azure/cosmosdb/item_get":          item_get.Icon,
		"azure/cosmosdb/item_get_all":      item_get_all.Icon,
		"azure/cosmosdb/item_patch":        item_patch.Icon,
		"azure/cosmosdb/item_query":        item_query.Icon,
		"azure/cosmosdb/item_replace":      item_replace.Icon,
		"azure/cosmosdb/throughput_get":    throughput_get.Icon,
		"azure/cosmosdb/throughput_update": throughput_update.Icon,
	}
}

// actionMetadata carries the per-action consts the manifest generator reads.
type actionMetadata struct {
	Author       string
	Organisation string
	Name         string
	Website      string
	Date         string
	Type         int
}

// cosmosActionMetadata backs the metadata-consistency assertion.
func cosmosActionMetadata() map[string]actionMetadata {
	return map[string]actionMetadata{
		"azure/cosmosdb/container_create":  {container_create.Author, container_create.Organisation, container_create.Name, container_create.Website, container_create.Date, container_create.Type},
		"azure/cosmosdb/container_delete":  {container_delete.Author, container_delete.Organisation, container_delete.Name, container_delete.Website, container_delete.Date, container_delete.Type},
		"azure/cosmosdb/container_get":     {container_get.Author, container_get.Organisation, container_get.Name, container_get.Website, container_get.Date, container_get.Type},
		"azure/cosmosdb/container_get_all": {container_get_all.Author, container_get_all.Organisation, container_get_all.Name, container_get_all.Website, container_get_all.Date, container_get_all.Type},
		"azure/cosmosdb/container_replace": {container_replace.Author, container_replace.Organisation, container_replace.Name, container_replace.Website, container_replace.Date, container_replace.Type},
		"azure/cosmosdb/database_create":   {database_create.Author, database_create.Organisation, database_create.Name, database_create.Website, database_create.Date, database_create.Type},
		"azure/cosmosdb/database_delete":   {database_delete.Author, database_delete.Organisation, database_delete.Name, database_delete.Website, database_delete.Date, database_delete.Type},
		"azure/cosmosdb/database_get":      {database_get.Author, database_get.Organisation, database_get.Name, database_get.Website, database_get.Date, database_get.Type},
		"azure/cosmosdb/database_get_all":  {database_get_all.Author, database_get_all.Organisation, database_get_all.Name, database_get_all.Website, database_get_all.Date, database_get_all.Type},
		"azure/cosmosdb/item_create":       {item_create.Author, item_create.Organisation, item_create.Name, item_create.Website, item_create.Date, item_create.Type},
		"azure/cosmosdb/item_delete":       {item_delete.Author, item_delete.Organisation, item_delete.Name, item_delete.Website, item_delete.Date, item_delete.Type},
		"azure/cosmosdb/item_get":          {item_get.Author, item_get.Organisation, item_get.Name, item_get.Website, item_get.Date, item_get.Type},
		"azure/cosmosdb/item_get_all":      {item_get_all.Author, item_get_all.Organisation, item_get_all.Name, item_get_all.Website, item_get_all.Date, item_get_all.Type},
		"azure/cosmosdb/item_patch":        {item_patch.Author, item_patch.Organisation, item_patch.Name, item_patch.Website, item_patch.Date, item_patch.Type},
		"azure/cosmosdb/item_query":        {item_query.Author, item_query.Organisation, item_query.Name, item_query.Website, item_query.Date, item_query.Type},
		"azure/cosmosdb/item_replace":      {item_replace.Author, item_replace.Organisation, item_replace.Name, item_replace.Website, item_replace.Date, item_replace.Type},
		"azure/cosmosdb/throughput_get":    {throughput_get.Author, throughput_get.Organisation, throughput_get.Name, throughput_get.Website, throughput_get.Date, throughput_get.Type},
		"azure/cosmosdb/throughput_update": {throughput_update.Author, throughput_update.Organisation, throughput_update.Name, throughput_update.Website, throughput_update.Date, throughput_update.Type},
	}
}

// cosmosBadges is the badge glyph set the Cosmos DB icons draw on. Every name
// here was checked against editor/app/components/icons/paths.ts (the `azure`
// base included) — a badge missing there renders as a silent "?" in the
// palette, which no compiler and no other test would catch. The spec's
// suggested "safe set" is NOT authoritative: it lists `tag`, which does not
// exist in paths.ts (the storage sibling found this the hard way), and omits
// `code` and `gauge`, which do.
var cosmosBadges = map[string]bool{
	"code":             true,
	"gauge":            true,
	"list":             true,
	"magnifying-glass": true,
	"pen":              true,
	"plus":             true,
	"rotate":           true,
	"trash":            true,
}

// cosmosListActions use results/count instead of id/result.
var cosmosListActions = map[string]bool{
	"azure/cosmosdb/container_get_all": true,
	"azure/cosmosdb/database_get_all":  true,
	"azure/cosmosdb/item_get_all":      true,
	"azure/cosmosdb/item_query":        true,
}

// TestCosmosAuthBlockDoesNotDrift is the whole reason this file exists.
//
// Every action's first eight inputs must reproduce cosmosdb.AuthInputs exactly
// — name, type, label, placeholder, required, options and visible_when, in the
// spec's order (account_name, auth_method, master_key, azure_tenant_id,
// azure_client_id, azure_client_secret, endpoint, allow_insecure). The
// visible_when values are the sharp edge: master_key shows when auth_method is
// "" or "master_key" (the empty string matters — it is what an unset dropdown
// reads as, so a fresh node shows the key field), and the azure_* fields only
// when it is "entra". A copy that drops the "" would hide the key field on a
// brand-new node, and GetAuth's `case "", AuthMethodMasterKey` would still
// demand the value the operator can no longer see.
func TestCosmosAuthBlockDoesNotDrift(t *testing.T) {
	want := cosmosdb.AuthInputs

	for id, inputs := range cosmosActionInputs() {
		if len(inputs) < len(want) {
			t.Errorf("%s: has %d inputs, fewer than the %d-field credential block", id, len(inputs), len(want))
			continue
		}
		for i := range want {
			if !reflect.DeepEqual(inputs[i], want[i]) {
				t.Errorf("%s: credential input %d (%q) has drifted from cosmosdb.AuthInputs\n got: %+v\nwant: %+v",
					id, i, want[i].Name, inputs[i], want[i])
			}
		}
	}
}

// TestCosmosAuthBlockOrderMatchesSpec pins the canonical order by name against
// the spec's literal list, independently of AuthInputs. Without this, editing
// AuthInputs and all 18 copies in the same sweep would keep the drift test
// green while silently rewriting the contract the api's dynamic-options Params
// lists depend on.
func TestCosmosAuthBlockOrderMatchesSpec(t *testing.T) {
	spec := []string{
		"account_name",
		"auth_method",
		"master_key",
		"azure_tenant_id",
		"azure_client_id",
		"azure_client_secret",
		"endpoint",
		"allow_insecure",
	}
	if len(cosmosdb.AuthInputs) != len(spec) {
		t.Fatalf("cosmosdb.AuthInputs has %d fields, spec pins %d", len(cosmosdb.AuthInputs), len(spec))
	}
	for i, name := range spec {
		if cosmosdb.AuthInputs[i].Name != name {
			t.Errorf("cosmosdb.AuthInputs[%d].Name = %q, spec pins %q", i, cosmosdb.AuthInputs[i].Name, name)
		}
	}
}

// TestCosmosAuthBlockTypesAndGating pins the field-level contract the spec
// spells out: the two secrets are Secret (not String — a String would render
// the master key in clear text and log it), allow_insecure is a Boolean, the
// auth_method dropdown offers exactly the two documented options, and only
// account_name is required (everything else is either method-gated or
// optional, so the editor cannot block a valid Entra config on a blank
// master_key).
func TestCosmosAuthBlockTypesAndGating(t *testing.T) {
	byName := map[string]core.Connection{}
	for _, c := range cosmosdb.AuthInputs {
		byName[c.Name] = c
	}

	for name, want := range map[string]string{
		"account_name":        core.ConnectionTypeString,
		"auth_method":         core.ConnectionTypeString,
		"master_key":          core.ConnectionTypeSecret,
		"azure_tenant_id":     core.ConnectionTypeString,
		"azure_client_id":     core.ConnectionTypeString,
		"azure_client_secret": core.ConnectionTypeSecret,
		"endpoint":            core.ConnectionTypeString,
		"allow_insecure":      core.ConnectionTypeBoolean,
	} {
		if got := byName[name].Type; got != want {
			t.Errorf("auth input %q has Type %v, want %v", name, got, want)
		}
	}

	if !byName["account_name"].Required {
		t.Error("account_name must be Required — every URL is built from it")
	}
	for _, name := range []string{"auth_method", "master_key", "azure_tenant_id", "azure_client_id", "azure_client_secret", "endpoint", "allow_insecure"} {
		if byName[name].Required {
			t.Errorf("auth input %q must NOT be Required — it is method-gated or optional; a hard Required flag blocks the other auth method", name)
		}
	}

	opts := byName["auth_method"].Options
	wantOpts := []core.ConnectionOption{
		{Name: "Master Key", Value: "master_key"},
		{Name: "Microsoft Entra (service principal)", Value: "entra"},
	}
	if !reflect.DeepEqual(opts, wantOpts) {
		t.Errorf("auth_method Options = %+v, want %+v", opts, wantOpts)
	}

	// master_key visible on "" (fresh node) and master_key; azure_* on entra.
	if v := byName["master_key"].Visible; v == nil ||
		v.Field != "auth_method" || !reflect.DeepEqual(v.Values, []string{"", "master_key"}) {
		t.Errorf("master_key Visible = %+v, want auth_method in [\"\", \"master_key\"] — dropping the empty string hides the key field on a brand-new node", v)
	}
	for _, name := range []string{"azure_tenant_id", "azure_client_id", "azure_client_secret"} {
		v := byName[name].Visible
		if v == nil || v.Field != "auth_method" || !reflect.DeepEqual(v.Values, []string{"entra"}) {
			t.Errorf("%s Visible = %+v, want auth_method in [\"entra\"]", name, v)
		}
	}
	for _, name := range []string{"account_name", "auth_method", "endpoint", "allow_insecure"} {
		if byName[name].Visible != nil {
			t.Errorf("%s must be unconditionally visible — it applies to both auth methods", name)
		}
	}
}

// TestCosmosNoResourceInputShadowsACredential guards the input-name collision
// core.FindConnection makes possible: it returns the FIRST input whose name
// matches, and the credential block is declared first — so a resource field
// that reuses a credential's name silently reads the CREDENTIAL instead. The
// live hazard here is `database`: it is a per-action resource input by design
// (the spec's headline fix over n8n), so it must never migrate into the auth
// block, and `container` likewise.
func TestCosmosNoResourceInputShadowsACredential(t *testing.T) {
	credential := map[string]bool{}
	for _, c := range cosmosdb.AuthInputs {
		credential[c.Name] = true
	}

	for id, inputs := range cosmosActionInputs() {
		if len(inputs) < len(cosmosdb.AuthInputs) {
			continue // TestCosmosAuthBlockDoesNotDrift already reports this
		}
		for _, c := range inputs[len(cosmosdb.AuthInputs):] {
			if credential[c.Name] {
				t.Errorf("%s: resource input %q shadows the credential input of the same name — "+
					"core.FindConnection would return the credential; rename it", id, c.Name)
			}
		}
	}
}

// TestCosmosNoDuplicateInputNames catches the same FindConnection hazard within
// the resource block itself — two inputs sharing a name means the second is
// unreachable and silently reads the first.
func TestCosmosNoDuplicateInputNames(t *testing.T) {
	for id, inputs := range cosmosActionInputs() {
		seen := map[string]bool{}
		for _, c := range inputs {
			if seen[c.Name] {
				t.Errorf("%s: input %q is declared twice — core.FindConnection returns the first, so the second is dead", id, c.Name)
			}
			seen[c.Name] = true
		}
	}
}

// TestCosmosIconsResolve keeps every icon inside the glyph set the editor
// actually ships: the azure base plus a badge from cosmosBadges.
func TestCosmosIconsResolve(t *testing.T) {
	for id, icon := range cosmosActionIcons() {
		base, badge, composed := strings.Cut(icon, "+")
		if !composed {
			t.Errorf("%s: icon %q is not a base+badge composition", id, icon)
			continue
		}
		if base != "azure" {
			t.Errorf("%s: icon base is %q, not \"azure\" — every action in this node wears the Azure mark", id, base)
		}
		if !cosmosBadges[badge] {
			t.Errorf("%s: icon badge %q is not in the verified paths.ts glyph set — it would render as \"?\" in the palette", id, badge)
		}
	}
}

// TestCosmosStandardOutputsPresent pins the baseline output contract IN ORDER,
// which the spec fixes exactly: id/result (or results/count for lists), then
// the extras, then tool_result, success, error. request_charge is an extra on
// every Cosmos op — the spec pins it node-wide, because RU cost is the number
// that decides whether a flow is affordable.
func TestCosmosStandardOutputsPresent(t *testing.T) {
	for id, outputs := range cosmosActionOutputs() {
		var want []string
		if cosmosListActions[id] {
			want = []string{"results", "count", "request_charge", "tool_result", "success", "error"}
		} else {
			want = []string{"id", "result", "request_charge", "tool_result", "success", "error"}
		}
		got := make([]string, 0, len(outputs))
		for _, o := range outputs {
			got = append(got, o.Name)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: outputs are %v, want %v (spec order: id/result or results/count, extras, tool_result, success, error)", id, got, want)
		}
	}
}

// TestCosmosStandardOutputTypes pins the types the platform reads: success
// must be Boolean (the soft-failure path branches on it) and tool_result must
// be the String the AI tool loop shows the model, labelled the way every other
// node labels it.
func TestCosmosStandardOutputTypes(t *testing.T) {
	for id, outputs := range cosmosActionOutputs() {
		for _, o := range outputs {
			switch o.Name {
			case "success":
				if o.Type != core.ConnectionTypeBoolean {
					t.Errorf("%s: success output has Type %v, want Boolean", id, o.Type)
				}
			case "error", "tool_result", "request_charge":
				if o.Type != core.ConnectionTypeString {
					t.Errorf("%s: %s output has Type %v, want String", id, o.Name, o.Type)
				}
			case "count":
				if o.Type != core.ConnectionTypeInteger {
					t.Errorf("%s: count output has Type %v, want Integer", id, o.Type)
				}
			case "result", "results":
				if o.Type != core.ConnectionTypeObject {
					t.Errorf("%s: %s output has Type %v, want Object", id, o.Name, o.Type)
				}
			}
			if o.Name == "tool_result" && o.Label != "Result Summary" {
				t.Errorf("%s: tool_result Label = %q, want \"Result Summary\"", id, o.Label)
			}
		}
	}
}

// TestCosmosMetadataIsConsistent pins the per-action consts. These are pure
// copy-paste with no compiler check, so one action carrying a stale Date or a
// typo'd Website is invisible until it reaches the palette.
func TestCosmosMetadataIsConsistent(t *testing.T) {
	for id, m := range cosmosActionMetadata() {
		if m.Author != "Dave McElin" {
			t.Errorf("%s: Author = %q, want \"Dave McElin\"", id, m.Author)
		}
		if m.Organisation != "Flomation" {
			t.Errorf("%s: Organisation = %q, want \"Flomation\"", id, m.Organisation)
		}
		if m.Website != "https://www.flomation.co" {
			t.Errorf("%s: Website = %q, want \"https://www.flomation.co\"", id, m.Website)
		}
		if m.Date != "16/07/2026" {
			t.Errorf("%s: Date = %q, want \"16/07/2026\"", id, m.Date)
		}
		if m.Type != core.ActionTypeAction {
			t.Errorf("%s: Type = %d, want ActionTypeAction (%d) — this node has no trigger in v1",
				id, m.Type, core.ActionTypeAction)
		}
		if !strings.HasPrefix(m.Name, "Cosmos DB: ") {
			t.Errorf("%s: Name = %q, want the \"Cosmos DB: \" prefix every action in this node shares", id, m.Name)
		}
	}
}

// TestCosmosDatabaseIsAPerActionInput is the spec's headline fix over n8n,
// which pins the database in the credential and so cannot touch two databases
// in one workflow. Every action that addresses a database takes it as a
// resource input, required, right after the credential block.
func TestCosmosDatabaseIsAPerActionInput(t *testing.T) {
	// database_create names the new database in `database` too, and
	// database_get_all lists them all — every other action addresses one.
	exempt := map[string]bool{"azure/cosmosdb/database_get_all": true}

	for id, inputs := range cosmosActionInputs() {
		if exempt[id] {
			continue
		}
		found := false
		for _, c := range inputs {
			if c.Name != "database" {
				continue
			}
			found = true
			if !c.Required {
				t.Errorf("%s: the %q input must be Required", id, c.Name)
			}
			if c.Type != core.ConnectionTypeString {
				t.Errorf("%s: the %q input has Type %v, want String", id, c.Name, c.Type)
			}
		}
		if !found {
			t.Errorf("%s: has no %q input — the database must never be pinned in the credential block", id, "database")
		}
	}
}

// TestCosmosListControlsUseConsistentLabels pins the paging-control
// convention. A non-technical operator learns Return All / Limit / Simplify
// once and must recognise them on every list action.
func TestCosmosListControlsUseConsistentLabels(t *testing.T) {
	canonical := map[string]string{
		"return_all": "Return All",
		"limit":      "Limit",
		"simplify":   "Simplify",
	}
	for id, inputs := range cosmosActionInputs() {
		for _, in := range inputs {
			if want, ok := canonical[in.Name]; ok && in.Label != want {
				t.Errorf("%s: %q Label = %q, want %q — the paging controls must read the same across every list action",
					id, in.Name, in.Label, want)
			}
		}
	}
}

// TestCosmosTableCoversEveryActionOnDisk pins the designed action count. If
// action 19 lands and nobody adds it to the tables here, this is what says so.
func TestCosmosTableCoversEveryActionOnDisk(t *testing.T) {
	const designed = 18
	if got := len(cosmosActionInputs()); got != designed {
		t.Errorf("cosmosActionInputs() covers %d actions, expected %d — a new Cosmos DB action must be added to the tables in this file", got, designed)
	}
	if got := len(cosmosActionOutputs()); got != designed {
		t.Errorf("cosmosActionOutputs() covers %d actions, expected %d", got, designed)
	}
	if got := len(cosmosActionIcons()); got != designed {
		t.Errorf("cosmosActionIcons() covers %d actions, expected %d", got, designed)
	}
	if got := len(cosmosActionMetadata()); got != designed {
		t.Errorf("cosmosActionMetadata() covers %d actions, expected %d", got, designed)
	}
}
