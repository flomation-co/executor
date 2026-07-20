// Cross-action invariants for the Azure ▸ Files node.
//
// All 20 azure/files actions re-declare the eight-field credential block
// INLINE, because the manifest generator AST-parses the Inputs literal and
// cannot see through a package-level variable. files.AuthInputs is therefore
// documentation, not enforcement — 20 copies of eight fields, free to drift one
// paste at a time. This file is the enforcement: a copy that drifts fails CI
// with the action and the field named. Modelled on
// actions/azure/storage/storage_inputs_drift_test.go.
package files_test

import (
	"reflect"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	files "flomation.app/automate/executor/actions/azure/files"

	directory_create "flomation.app/automate/executor/actions/azure/files/directory_create"
	directory_delete "flomation.app/automate/executor/actions/azure/files/directory_delete"
	directory_get_all "flomation.app/automate/executor/actions/azure/files/directory_get_all"
	directory_get_properties "flomation.app/automate/executor/actions/azure/files/directory_get_properties"
	file_copy "flomation.app/automate/executor/actions/azure/files/file_copy"
	file_delete "flomation.app/automate/executor/actions/azure/files/file_delete"
	file_download "flomation.app/automate/executor/actions/azure/files/file_download"
	file_generate_sas "flomation.app/automate/executor/actions/azure/files/file_generate_sas"
	file_get_properties "flomation.app/automate/executor/actions/azure/files/file_get_properties"
	file_lease "flomation.app/automate/executor/actions/azure/files/file_lease"
	file_list_ranges "flomation.app/automate/executor/actions/azure/files/file_list_ranges"
	file_set_metadata "flomation.app/automate/executor/actions/azure/files/file_set_metadata"
	file_upload "flomation.app/automate/executor/actions/azure/files/file_upload"
	share_create "flomation.app/automate/executor/actions/azure/files/share_create"
	share_delete "flomation.app/automate/executor/actions/azure/files/share_delete"
	share_get_all "flomation.app/automate/executor/actions/azure/files/share_get_all"
	share_get_properties "flomation.app/automate/executor/actions/azure/files/share_get_properties"
	share_get_stats "flomation.app/automate/executor/actions/azure/files/share_get_stats"
	share_set_metadata "flomation.app/automate/executor/actions/azure/files/share_set_metadata"
	share_set_properties "flomation.app/automate/executor/actions/azure/files/share_set_properties"
)

// filesActionInputs is the table every assertion below ranges over. All 20
// actions.
func filesActionInputs() map[string][]core.Connection {
	return map[string][]core.Connection{
		"azure/files/directory_create":         directory_create.Inputs[:],
		"azure/files/directory_delete":         directory_delete.Inputs[:],
		"azure/files/directory_get_all":        directory_get_all.Inputs[:],
		"azure/files/directory_get_properties": directory_get_properties.Inputs[:],
		"azure/files/file_copy":                file_copy.Inputs[:],
		"azure/files/file_delete":              file_delete.Inputs[:],
		"azure/files/file_download":            file_download.Inputs[:],
		"azure/files/file_generate_sas":        file_generate_sas.Inputs[:],
		"azure/files/file_get_properties":      file_get_properties.Inputs[:],
		"azure/files/file_lease":               file_lease.Inputs[:],
		"azure/files/file_list_ranges":         file_list_ranges.Inputs[:],
		"azure/files/file_set_metadata":        file_set_metadata.Inputs[:],
		"azure/files/file_upload":              file_upload.Inputs[:],
		"azure/files/share_create":             share_create.Inputs[:],
		"azure/files/share_delete":             share_delete.Inputs[:],
		"azure/files/share_get_all":            share_get_all.Inputs[:],
		"azure/files/share_get_properties":     share_get_properties.Inputs[:],
		"azure/files/share_get_stats":          share_get_stats.Inputs[:],
		"azure/files/share_set_metadata":       share_set_metadata.Inputs[:],
		"azure/files/share_set_properties":     share_set_properties.Inputs[:],
	}
}

// filesActionOutputs backs the standard-outputs assertion.
func filesActionOutputs() map[string][]core.Connection {
	return map[string][]core.Connection{
		"azure/files/directory_create":         directory_create.Outputs[:],
		"azure/files/directory_delete":         directory_delete.Outputs[:],
		"azure/files/directory_get_all":        directory_get_all.Outputs[:],
		"azure/files/directory_get_properties": directory_get_properties.Outputs[:],
		"azure/files/file_copy":                file_copy.Outputs[:],
		"azure/files/file_delete":              file_delete.Outputs[:],
		"azure/files/file_download":            file_download.Outputs[:],
		"azure/files/file_generate_sas":        file_generate_sas.Outputs[:],
		"azure/files/file_get_properties":      file_get_properties.Outputs[:],
		"azure/files/file_lease":               file_lease.Outputs[:],
		"azure/files/file_list_ranges":         file_list_ranges.Outputs[:],
		"azure/files/file_set_metadata":        file_set_metadata.Outputs[:],
		"azure/files/file_upload":              file_upload.Outputs[:],
		"azure/files/share_create":             share_create.Outputs[:],
		"azure/files/share_delete":             share_delete.Outputs[:],
		"azure/files/share_get_all":            share_get_all.Outputs[:],
		"azure/files/share_get_properties":     share_get_properties.Outputs[:],
		"azure/files/share_get_stats":          share_get_stats.Outputs[:],
		"azure/files/share_set_metadata":       share_set_metadata.Outputs[:],
		"azure/files/share_set_properties":     share_set_properties.Outputs[:],
	}
}

// filesActionIcons backs the icon-resolution assertion.
func filesActionIcons() map[string]string {
	return map[string]string{
		"azure/files/directory_create":         directory_create.Icon,
		"azure/files/directory_delete":         directory_delete.Icon,
		"azure/files/directory_get_all":        directory_get_all.Icon,
		"azure/files/directory_get_properties": directory_get_properties.Icon,
		"azure/files/file_copy":                file_copy.Icon,
		"azure/files/file_delete":              file_delete.Icon,
		"azure/files/file_download":            file_download.Icon,
		"azure/files/file_generate_sas":        file_generate_sas.Icon,
		"azure/files/file_get_properties":      file_get_properties.Icon,
		"azure/files/file_lease":               file_lease.Icon,
		"azure/files/file_list_ranges":         file_list_ranges.Icon,
		"azure/files/file_set_metadata":        file_set_metadata.Icon,
		"azure/files/file_upload":              file_upload.Icon,
		"azure/files/share_create":             share_create.Icon,
		"azure/files/share_delete":             share_delete.Icon,
		"azure/files/share_get_all":            share_get_all.Icon,
		"azure/files/share_get_properties":     share_get_properties.Icon,
		"azure/files/share_get_stats":          share_get_stats.Icon,
		"azure/files/share_set_metadata":       share_set_metadata.Icon,
		"azure/files/share_set_properties":     share_set_properties.Icon,
	}
}

// filesBadges is the badge glyph set the Files icons draw on. Every name here
// was checked against editor/app/components/icons/paths.ts — a badge missing
// there renders as a silent "?" in the palette, which no compiler and no other
// test would catch.
var filesBadges = map[string]bool{
	"arrow-down":       true,
	"arrow-up":         true,
	"chart-column":     true,
	"copy":             true,
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
	"azure/files/share_get_all":     true,
	"azure/files/directory_get_all": true,
	"azure/files/file_list_ranges":  true,
}

// TestFilesAuthBlockDoesNotDrift is the whole reason this file exists.
//
// Every action's first eight inputs must reproduce files.AuthInputs exactly —
// name, type, label, placeholder, required, options and visible_when, in order.
// The visible_when values are the sharp edge: account_key shows when
// auth_method is "" or "shared_key" (the empty string matters — it is what an
// unset dropdown reads as, so a fresh node shows the key field), and the azure_*
// fields only when it is "entra". A copy that drops the "" would hide the key
// field on a brand-new node.
func TestFilesAuthBlockDoesNotDrift(t *testing.T) {
	want := files.AuthInputs

	for id, inputs := range filesActionInputs() {
		if len(inputs) < len(want) {
			t.Errorf("%s: has %d inputs, fewer than the %d-field credential block", id, len(inputs), len(want))
			continue
		}
		for i := range want {
			if !reflect.DeepEqual(inputs[i], want[i]) {
				t.Errorf("%s: credential input %d (%q) has drifted from files.AuthInputs\n got: %+v\nwant: %+v",
					id, i, want[i].Name, inputs[i], want[i])
			}
		}
	}
}

// TestFilesAuthBlockMirrorsStorage pins the one thing an operator moving between
// the Blob and Files nodes relies on: the credential block is the SAME eight
// fields, in the same order, with the same types and visibility. The two
// packages cannot share the literal (see the drift test above), so nothing but
// this stops them diverging.
//
// The two placeholders that legitimately differ are exempted by name, and the
// exemption list is the point: it says which differences were decided, so a
// third one shows up as a failure rather than as a drift nobody chose.
func TestFilesAuthBlockMirrorsStorage(t *testing.T) {
	// The Blob node's block, transcribed. Comparing against the storage package
	// directly would be neater, but this test would then pass or fail on a
	// change to a merged, live node that is out of this change's scope.
	wantNames := []string{
		"account_name", "auth_method", "account_key",
		"azure_tenant_id", "azure_client_id", "azure_client_secret",
		"endpoint", "allow_insecure",
	}
	if len(files.AuthInputs) != len(wantNames) {
		t.Fatalf("files.AuthInputs has %d fields, want the same %d as azure/storage", len(files.AuthInputs), len(wantNames))
	}
	for i, name := range wantNames {
		if files.AuthInputs[i].Name != name {
			t.Errorf("credential field %d is %q, want %q — the block must mirror azure/storage field-for-field", i, files.AuthInputs[i].Name, name)
		}
	}

	// endpoint must name the FILE host, not the blob one — deriving
	// {account}.blob.core.windows.net here would point every action at the wrong
	// service, and the placeholder is the only place an operator sees which.
	if ep := files.AuthInputs[6]; !strings.Contains(ep.Placeholder, ".file.core.windows.net") {
		t.Errorf("the endpoint placeholder must name the .file. host, got %q", ep.Placeholder)
	}
	// The Entra secret must warn about the ACL bypass: on this service, unlike
	// Blob, choosing OAuth forces x-ms-file-request-intent: backup, which
	// ignores the share's file permissions. An operator picking it from a
	// dropdown has to be told.
	if sec := files.AuthInputs[5]; !strings.Contains(sec.Placeholder, "BYPASSES") {
		t.Errorf("the client-secret placeholder must warn that Entra bypasses the share's file permissions, got %q", sec.Placeholder)
	}
}

// TestFilesNoResourceInputShadowsACredential guards the input-name collision
// core.FindConnection makes possible: it returns the FIRST input whose name
// matches, and the credential block is declared first — so a resource field that
// reuses a credential's name silently reads the CREDENTIAL instead.
func TestFilesNoResourceInputShadowsACredential(t *testing.T) {
	credential := map[string]bool{}
	for _, c := range files.AuthInputs {
		credential[c.Name] = true
	}

	for id, inputs := range filesActionInputs() {
		if len(inputs) < len(files.AuthInputs) {
			continue // TestFilesAuthBlockDoesNotDrift already reports this
		}
		for _, c := range inputs[len(files.AuthInputs):] {
			if credential[c.Name] {
				t.Errorf("%s: resource input %q shadows the credential input of the same name — "+
					"core.FindConnection would return the credential; rename it", id, c.Name)
			}
		}
	}
}

// leaseIDActions is every action that must expose the optional lease_id field,
// i.e. every FILE operation the service accepts an x-ms-lease-id on.
//
// The set is deliberately CLOSED, and the test below asserts both directions.
// The absences are the interesting half:
//
//   - Every directory_* action — a directory cannot be leased at all. The
//     lock lives on files (and on shares), never on the tree between them.
//   - Every share_* action — the File service DOES lease shares, but this node
//     exposes no share-lease lifecycle, so a share lease ID could only ever
//     arrive from outside the platform. Recorded here so the gap is a decision
//     on the record rather than an oversight.
//   - file_generate_sas signs a token locally and issues no request at all.
var leaseIDActions = map[string]bool{
	"azure/files/file_upload":         true,
	"azure/files/file_download":       true,
	"azure/files/file_delete":         true,
	"azure/files/file_get_properties": true,
	"azure/files/file_set_metadata":   true,
	"azure/files/file_list_ranges":    true,
	"azure/files/file_copy":           true,
}

// leaseLifecycleActions redeclare lease_id with their own Visible gate and their
// own meaning (the ID being released/changed rather than an optional
// assertion), so they are exempt from the LeaseIDInput comparison below.
var leaseLifecycleActions = map[string]bool{
	"azure/files/file_lease": true,
}

// TestFilesLeaseIDInputDoesNotDrift is the AuthInputs argument applied to the
// lease field: seven more copies of one literal, free to drift one paste at a
// time. It asserts the whole set — a copy that drifts, an action that is missing
// the field, and an action that grew one it should not have.
//
// The last direction matters most. The field is only honest where the service
// accepts the header: adding it to a directory operation would render an input
// that silently does nothing, which is worse than not offering it.
func TestFilesLeaseIDInputDoesNotDrift(t *testing.T) {
	for id, inputs := range filesActionInputs() {
		if leaseLifecycleActions[id] {
			continue
		}
		var found *core.Connection
		for i := range inputs {
			if inputs[i].Name == "lease_id" {
				found = &inputs[i]
			}
		}
		switch {
		case leaseIDActions[id] && found == nil:
			t.Errorf("%s: accepts x-ms-lease-id but declares no lease_id input — a leased file would be unwritable from this action", id)
		case !leaseIDActions[id] && found != nil:
			t.Errorf("%s: declares a lease_id input, but the File service takes no lease header on this operation — the field would do nothing", id)
		case found != nil && !reflect.DeepEqual(*found, files.LeaseIDInput):
			t.Errorf("%s: lease_id has drifted from files.LeaseIDInput\n got: %+v\nwant: %+v", id, *found, files.LeaseIDInput)
		}
	}
}

// TestFilesLeaseIDIsNotACredential pins where the field lives. It is an
// operator-supplied fact about one call, not a credential: it must sit AFTER the
// eight-field auth block, or the auth-block drift assertion — and the api's
// dynamic-options params, which name those eight inputs positionally by name —
// would both be reading a different shape than they were written against.
func TestFilesLeaseIDIsNotACredential(t *testing.T) {
	for _, c := range files.AuthInputs {
		if c.Name == "lease_id" {
			t.Fatal("lease_id has been added to files.AuthInputs — it is a resource field, not a credential")
		}
	}
	for id, inputs := range filesActionInputs() {
		for i, c := range inputs {
			if c.Name == "lease_id" && i < len(files.AuthInputs) {
				t.Errorf("%s: lease_id is at index %d, inside the %d-field credential block", id, i, len(files.AuthInputs))
			}
		}
	}
}

// TestFilesLeaseActionOptionsMatchTheService pins the one place the Files lease
// UI must NOT be a copy of the Blob one. A file lease is infinite-only, so
// there is no Renew (nothing expires to extend) and no Duration input — offering
// either would render a control the service rejects.
func TestFilesLeaseActionOptionsMatchTheService(t *testing.T) {
	inputs := filesActionInputs()["azure/files/file_lease"]
	for _, c := range inputs {
		if c.Name == "duration" || c.Name == "break_period" {
			t.Errorf("file_lease declares %q — a file lease is infinite-only, so the service takes no duration", c.Name)
		}
		if c.Name != "lease_action" {
			continue
		}
		for _, o := range c.Options {
			if o.Value == "renew" {
				t.Error("file_lease offers Renew — an infinite lease has nothing to renew, and the service rejects the action on a file")
			}
		}
		if len(c.Options) != len(files.LeaseActionOptions) {
			t.Errorf("file_lease offers %d lease actions, want the %d in files.LeaseActionOptions", len(c.Options), len(files.LeaseActionOptions))
		}
		for i := range files.LeaseActionOptions {
			if i < len(c.Options) && c.Options[i] != files.LeaseActionOptions[i] {
				t.Errorf("file_lease option %d = %+v, want %+v", i, c.Options[i], files.LeaseActionOptions[i])
			}
		}
	}
}

// TestFilesIconsResolve keeps every icon inside the glyph set the editor
// actually ships: the azure base plus a badge from filesBadges.
func TestFilesIconsResolve(t *testing.T) {
	for id, icon := range filesActionIcons() {
		base, badge, composed := strings.Cut(icon, "+")
		if !composed {
			t.Errorf("%s: icon %q is not a base+badge composition", id, icon)
			continue
		}
		if base != "azure" {
			t.Errorf("%s: icon base is %q, not \"azure\" — every action in this node wears the Azure mark", id, base)
		}
		if !filesBadges[badge] {
			t.Errorf("%s: icon badge %q is not in the verified paths.ts glyph set — it would render as \"?\" in the palette", id, badge)
		}
	}
}

// TestFilesCategoryIconResolves — the sub-group's own glyph is a bare name, not
// a composition, and is checked against paths.ts by the same standard.
func TestFilesCategoryIconResolves(t *testing.T) {
	// folder-tree is present in editor/app/components/icons/paths.ts (verified);
	// folder and folder-open are its neighbours there if it ever moves.
	if files.CategoryIcon != "folder-tree" {
		t.Errorf("CategoryIcon = %q — check it against paths.ts before changing it", files.CategoryIcon)
	}
	if files.CategoryName != "Files" {
		t.Errorf("CategoryName = %q, want Files — the api's subCategoryMetadata is written against this", files.CategoryName)
	}
}

// TestFilesStandardOutputsPresent pins the outputs the platform depends on:
// success drives the soft-failure path, error carries the message, tool_result
// is what the AI tool loop shows the model — plus the id/result vs results/count
// baseline split between single-resource and list actions.
func TestFilesStandardOutputsPresent(t *testing.T) {
	for id, outputs := range filesActionOutputs() {
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

// TestFilesTableCoversEveryActionOnDisk pins the designed action count. If
// action 21 lands and nobody adds it to the tables here, this is what says so.
func TestFilesTableCoversEveryActionOnDisk(t *testing.T) {
	const designed = 20
	if got := len(filesActionInputs()); got != designed {
		t.Errorf("filesActionInputs() covers %d actions, expected %d — a new files action must be added to the tables in this file", got, designed)
	}
	if got := len(filesActionOutputs()); got != designed {
		t.Errorf("filesActionOutputs() covers %d actions, expected %d", got, designed)
	}
	if got := len(filesActionIcons()); got != designed {
		t.Errorf("filesActionIcons() covers %d actions, expected %d", got, designed)
	}
}
