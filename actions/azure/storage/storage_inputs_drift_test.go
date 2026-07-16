// Cross-action invariants for the Azure ▸ Storage node.
//
// All 21 azure/storage actions re-declare the eight-field credential block
// INLINE, because the manifest generator AST-parses the Inputs literal and
// cannot see through a package-level variable. storage.AuthInputs is therefore
// documentation, not enforcement — 21 copies of eight fields, free to drift
// one paste at a time. This file is the enforcement: a copy that drifts fails
// CI with the action and the field named. Modelled on
// actions/infrastructure/awx_inputs_drift_test.go.
package storage_test

import (
	"reflect"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	storage "flomation.app/automate/executor/actions/azure/storage"

	blob_copy "flomation.app/automate/executor/actions/azure/storage/blob_copy"
	blob_delete "flomation.app/automate/executor/actions/azure/storage/blob_delete"
	blob_download "flomation.app/automate/executor/actions/azure/storage/blob_download"
	blob_find_by_tags "flomation.app/automate/executor/actions/azure/storage/blob_find_by_tags"
	blob_generate_sas "flomation.app/automate/executor/actions/azure/storage/blob_generate_sas"
	blob_get_all "flomation.app/automate/executor/actions/azure/storage/blob_get_all"
	blob_get_properties "flomation.app/automate/executor/actions/azure/storage/blob_get_properties"
	blob_get_tags "flomation.app/automate/executor/actions/azure/storage/blob_get_tags"
	blob_set_metadata "flomation.app/automate/executor/actions/azure/storage/blob_set_metadata"
	blob_set_properties "flomation.app/automate/executor/actions/azure/storage/blob_set_properties"
	blob_set_tags "flomation.app/automate/executor/actions/azure/storage/blob_set_tags"
	blob_set_tier "flomation.app/automate/executor/actions/azure/storage/blob_set_tier"
	blob_snapshot "flomation.app/automate/executor/actions/azure/storage/blob_snapshot"
	blob_undelete "flomation.app/automate/executor/actions/azure/storage/blob_undelete"
	blob_upload "flomation.app/automate/executor/actions/azure/storage/blob_upload"
	blob_upload_from_url "flomation.app/automate/executor/actions/azure/storage/blob_upload_from_url"
	container_create "flomation.app/automate/executor/actions/azure/storage/container_create"
	container_delete "flomation.app/automate/executor/actions/azure/storage/container_delete"
	container_get "flomation.app/automate/executor/actions/azure/storage/container_get"
	container_get_all "flomation.app/automate/executor/actions/azure/storage/container_get_all"
	container_set_metadata "flomation.app/automate/executor/actions/azure/storage/container_set_metadata"
)

// storageActionInputs is the table every assertion below ranges over. All 21
// actions.
func storageActionInputs() map[string][]core.Connection {
	return map[string][]core.Connection{
		"azure/storage/blob_copy":              blob_copy.Inputs[:],
		"azure/storage/blob_delete":            blob_delete.Inputs[:],
		"azure/storage/blob_download":          blob_download.Inputs[:],
		"azure/storage/blob_find_by_tags":      blob_find_by_tags.Inputs[:],
		"azure/storage/blob_generate_sas":      blob_generate_sas.Inputs[:],
		"azure/storage/blob_get_all":           blob_get_all.Inputs[:],
		"azure/storage/blob_get_properties":    blob_get_properties.Inputs[:],
		"azure/storage/blob_get_tags":          blob_get_tags.Inputs[:],
		"azure/storage/blob_set_metadata":      blob_set_metadata.Inputs[:],
		"azure/storage/blob_set_properties":    blob_set_properties.Inputs[:],
		"azure/storage/blob_set_tags":          blob_set_tags.Inputs[:],
		"azure/storage/blob_set_tier":          blob_set_tier.Inputs[:],
		"azure/storage/blob_snapshot":          blob_snapshot.Inputs[:],
		"azure/storage/blob_undelete":          blob_undelete.Inputs[:],
		"azure/storage/blob_upload":            blob_upload.Inputs[:],
		"azure/storage/blob_upload_from_url":   blob_upload_from_url.Inputs[:],
		"azure/storage/container_create":       container_create.Inputs[:],
		"azure/storage/container_delete":       container_delete.Inputs[:],
		"azure/storage/container_get":          container_get.Inputs[:],
		"azure/storage/container_get_all":      container_get_all.Inputs[:],
		"azure/storage/container_set_metadata": container_set_metadata.Inputs[:],
	}
}

// storageActionOutputs backs the standard-outputs assertion.
func storageActionOutputs() map[string][]core.Connection {
	return map[string][]core.Connection{
		"azure/storage/blob_copy":              blob_copy.Outputs[:],
		"azure/storage/blob_delete":            blob_delete.Outputs[:],
		"azure/storage/blob_download":          blob_download.Outputs[:],
		"azure/storage/blob_find_by_tags":      blob_find_by_tags.Outputs[:],
		"azure/storage/blob_generate_sas":      blob_generate_sas.Outputs[:],
		"azure/storage/blob_get_all":           blob_get_all.Outputs[:],
		"azure/storage/blob_get_properties":    blob_get_properties.Outputs[:],
		"azure/storage/blob_get_tags":          blob_get_tags.Outputs[:],
		"azure/storage/blob_set_metadata":      blob_set_metadata.Outputs[:],
		"azure/storage/blob_set_properties":    blob_set_properties.Outputs[:],
		"azure/storage/blob_set_tags":          blob_set_tags.Outputs[:],
		"azure/storage/blob_set_tier":          blob_set_tier.Outputs[:],
		"azure/storage/blob_snapshot":          blob_snapshot.Outputs[:],
		"azure/storage/blob_undelete":          blob_undelete.Outputs[:],
		"azure/storage/blob_upload":            blob_upload.Outputs[:],
		"azure/storage/blob_upload_from_url":   blob_upload_from_url.Outputs[:],
		"azure/storage/container_create":       container_create.Outputs[:],
		"azure/storage/container_delete":       container_delete.Outputs[:],
		"azure/storage/container_get":          container_get.Outputs[:],
		"azure/storage/container_get_all":      container_get_all.Outputs[:],
		"azure/storage/container_set_metadata": container_set_metadata.Outputs[:],
	}
}

// storageActionIcons backs the icon-resolution assertion.
func storageActionIcons() map[string]string {
	return map[string]string{
		"azure/storage/blob_copy":              blob_copy.Icon,
		"azure/storage/blob_delete":            blob_delete.Icon,
		"azure/storage/blob_download":          blob_download.Icon,
		"azure/storage/blob_find_by_tags":      blob_find_by_tags.Icon,
		"azure/storage/blob_generate_sas":      blob_generate_sas.Icon,
		"azure/storage/blob_get_all":           blob_get_all.Icon,
		"azure/storage/blob_get_properties":    blob_get_properties.Icon,
		"azure/storage/blob_get_tags":          blob_get_tags.Icon,
		"azure/storage/blob_set_metadata":      blob_set_metadata.Icon,
		"azure/storage/blob_set_properties":    blob_set_properties.Icon,
		"azure/storage/blob_set_tags":          blob_set_tags.Icon,
		"azure/storage/blob_set_tier":          blob_set_tier.Icon,
		"azure/storage/blob_snapshot":          blob_snapshot.Icon,
		"azure/storage/blob_undelete":          blob_undelete.Icon,
		"azure/storage/blob_upload":            blob_upload.Icon,
		"azure/storage/blob_upload_from_url":   blob_upload_from_url.Icon,
		"azure/storage/container_create":       container_create.Icon,
		"azure/storage/container_delete":       container_delete.Icon,
		"azure/storage/container_get":          container_get.Icon,
		"azure/storage/container_get_all":      container_get_all.Icon,
		"azure/storage/container_set_metadata": container_set_metadata.Icon,
	}
}

// storageBadges is the badge glyph set the storage icons draw on. Every name
// here was checked against editor/app/components/icons/paths.ts — a badge
// missing there renders as a silent "?" in the palette, which no compiler and
// no other test would catch. (tag/tags do NOT exist in paths.ts, which is why
// the tag actions wear hashtag.)
var storageBadges = map[string]bool{
	"arrow-down":        true,
	"arrow-up":          true,
	"clock-rotate-left": true,
	"copy":              true,
	"gear":              true,
	"hashtag":           true,
	"key":               true,
	"layer-group":       true,
	"link":              true,
	"list":              true,
	"magnifying-glass":  true,
	"pen":               true,
	"plus":              true,
	"rotate-left":       true,
	"trash":             true,
}

// listActions use results/count instead of id/result.
var listActions = map[string]bool{
	"azure/storage/blob_find_by_tags": true,
	"azure/storage/blob_get_all":      true,
	"azure/storage/container_get_all": true,
}

// TestStorageAuthBlockDoesNotDrift is the whole reason this file exists.
//
// Every action's first eight inputs must reproduce storage.AuthInputs exactly
// — name, type, label, placeholder, required, options and visible_when, in
// order. The visible_when values are the sharp edge: account_key shows when
// auth_method is "" or "shared_key" (the empty string matters — it is what an
// unset dropdown reads as, so a fresh node shows the key field), and the
// azure_* fields only when it is "entra". A copy that drops the "" would hide
// the key field on a brand-new node.
func TestStorageAuthBlockDoesNotDrift(t *testing.T) {
	want := storage.AuthInputs

	for id, inputs := range storageActionInputs() {
		if len(inputs) < len(want) {
			t.Errorf("%s: has %d inputs, fewer than the %d-field credential block", id, len(inputs), len(want))
			continue
		}
		for i := range want {
			if !reflect.DeepEqual(inputs[i], want[i]) {
				t.Errorf("%s: credential input %d (%q) has drifted from storage.AuthInputs\n got: %+v\nwant: %+v",
					id, i, want[i].Name, inputs[i], want[i])
			}
		}
	}
}

// TestStorageNoResourceInputShadowsACredential guards the input-name collision
// core.FindConnection makes possible: it returns the FIRST input whose name
// matches, and the credential block is declared first — so a resource field
// that reuses a credential's name silently reads the CREDENTIAL instead.
func TestStorageNoResourceInputShadowsACredential(t *testing.T) {
	credential := map[string]bool{}
	for _, c := range storage.AuthInputs {
		credential[c.Name] = true
	}

	for id, inputs := range storageActionInputs() {
		if len(inputs) < len(storage.AuthInputs) {
			continue // TestStorageAuthBlockDoesNotDrift already reports this
		}
		for _, c := range inputs[len(storage.AuthInputs):] {
			if credential[c.Name] {
				t.Errorf("%s: resource input %q shadows the credential input of the same name — "+
					"core.FindConnection would return the credential; rename it", id, c.Name)
			}
		}
	}
}

// TestStorageIconsResolve keeps every icon inside the glyph set the editor
// actually ships: the azure base plus a badge from storageBadges.
func TestStorageIconsResolve(t *testing.T) {
	for id, icon := range storageActionIcons() {
		base, badge, composed := strings.Cut(icon, "+")
		if !composed {
			t.Errorf("%s: icon %q is not a base+badge composition", id, icon)
			continue
		}
		if base != "azure" {
			t.Errorf("%s: icon base is %q, not \"azure\" — every action in this node wears the Azure mark", id, base)
		}
		if !storageBadges[badge] {
			t.Errorf("%s: icon badge %q is not in the verified paths.ts glyph set — it would render as \"?\" in the palette", id, badge)
		}
	}
}

// TestStorageStandardOutputsPresent pins the outputs the platform depends on:
// success drives the soft-failure path, error carries the message, tool_result
// is what the AI tool loop shows the model — plus the id/result vs
// results/count baseline split between single-resource and list actions.
func TestStorageStandardOutputsPresent(t *testing.T) {
	for id, outputs := range storageActionOutputs() {
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

// TestStorageTableCoversEveryActionOnDisk pins the designed action count. If
// action 22 lands and nobody adds it to the tables here, this is what says so.
func TestStorageTableCoversEveryActionOnDisk(t *testing.T) {
	const designed = 21
	if got := len(storageActionInputs()); got != designed {
		t.Errorf("storageActionInputs() covers %d actions, expected %d — a new storage action must be added to the tables in this file", got, designed)
	}
	if got := len(storageActionOutputs()); got != designed {
		t.Errorf("storageActionOutputs() covers %d actions, expected %d", got, designed)
	}
	if got := len(storageActionIcons()); got != designed {
		t.Errorf("storageActionIcons() covers %d actions, expected %d", got, designed)
	}
}
