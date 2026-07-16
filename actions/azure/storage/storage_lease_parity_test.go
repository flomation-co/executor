// Lease-ID parity across the azure/storage node.
//
// storage_inputs_drift_test.go asserts every action that should expose
// lease_id DECLARES it. This file asserts the other half — that declaring it
// actually sends it — by driving each action's real Execute against a capture
// server, twice: once with a lease ID set, once without.
//
// Both directions are the test. A header that is never sent makes the input a
// lie; a header sent EMPTY when the operator left the field blank is worse
// than either, because "" is not "no lease" to the Blob service, it is an
// invalid lease, and every unleased call would start failing with 400.
package storage_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	core "flomation.app/automate/executor"

	blob_delete "flomation.app/automate/executor/actions/azure/storage/blob_delete"
	blob_download "flomation.app/automate/executor/actions/azure/storage/blob_download"
	blob_get_properties "flomation.app/automate/executor/actions/azure/storage/blob_get_properties"
	blob_get_tags "flomation.app/automate/executor/actions/azure/storage/blob_get_tags"
	blob_set_metadata "flomation.app/automate/executor/actions/azure/storage/blob_set_metadata"
	blob_set_properties "flomation.app/automate/executor/actions/azure/storage/blob_set_properties"
	blob_set_tags "flomation.app/automate/executor/actions/azure/storage/blob_set_tags"
	blob_set_tier "flomation.app/automate/executor/actions/azure/storage/blob_set_tier"
	blob_upload "flomation.app/automate/executor/actions/azure/storage/blob_upload"
	container_delete "flomation.app/automate/executor/actions/azure/storage/container_delete"
	container_get "flomation.app/automate/executor/actions/azure/storage/container_get"
	container_set_metadata "flomation.app/automate/executor/actions/azure/storage/container_set_metadata"
)

const parityTestKey = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="

// theLeaseID is a well-formed lease GUID, as Azure would have minted it.
const theLeaseID = "8b1c6a2e-0f9d-4a3b-9c5e-7d2f1a4b6c8d"

type executeFunc func(*core.Flow, *core.Node, []*core.Connection) (map[string]interface{}, error)

type leaseParityCase struct {
	id      string
	execute executeFunc
	// resource is the action's required non-credential inputs, lease_id aside.
	resource []*core.Connection
	// respond canned-answers the action so it reaches its success path; a nil
	// respond means a bare 200.
	respond func(http.ResponseWriter)
}

func s(name, v string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: v}
}

func o(name, v string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeObject, Value: v}
}

// leaseParityCases covers every action in leaseIDActions. The two tables are
// cross-checked below, so an action added to one and forgotten in the other
// fails rather than passing silently.
func leaseParityCases() []leaseParityCase {
	blob := []*core.Connection{s("container", "my-container"), s("blob_name", "hello.txt")}
	withBlob := func(extra ...*core.Connection) []*core.Connection {
		return append(append([]*core.Connection{}, blob...), extra...)
	}
	return []leaseParityCase{
		{id: "azure/storage/blob_upload", execute: blob_upload.Execute,
			resource: withBlob(s("content", "hi"))},
		{id: "azure/storage/blob_download", execute: blob_download.Execute,
			resource: blob,
			respond: func(w http.ResponseWriter) {
				w.Header().Set("Content-Type", "text/plain")
				_, _ = w.Write([]byte("hi"))
			}},
		{id: "azure/storage/blob_delete", execute: blob_delete.Execute, resource: blob},
		{id: "azure/storage/blob_get_properties", execute: blob_get_properties.Execute, resource: blob},
		{id: "azure/storage/blob_set_metadata", execute: blob_set_metadata.Execute,
			resource: withBlob(o("metadata", `{"reviewed":"true"}`))},
		{id: "azure/storage/blob_set_properties", execute: blob_set_properties.Execute,
			resource: withBlob(s("content_type", "text/plain"))},
		{id: "azure/storage/blob_set_tier", execute: blob_set_tier.Execute,
			resource: withBlob(s("access_tier", "Cool"))},
		{id: "azure/storage/blob_get_tags", execute: blob_get_tags.Execute,
			resource: blob,
			respond: func(w http.ResponseWriter) {
				_, _ = w.Write([]byte(`<?xml version="1.0"?><Tags><TagSet><Tag><Key>project</Key><Value>alpha</Value></Tag></TagSet></Tags>`))
			}},
		{id: "azure/storage/blob_set_tags", execute: blob_set_tags.Execute,
			resource: withBlob(o("tags", `{"project":"alpha"}`))},
		{id: "azure/storage/container_delete", execute: container_delete.Execute,
			resource: []*core.Connection{s("container", "my-container")}},
		{id: "azure/storage/container_get", execute: container_get.Execute,
			resource: []*core.Connection{s("container", "my-container")}},
		{id: "azure/storage/container_set_metadata", execute: container_set_metadata.Execute,
			resource: []*core.Connection{s("container", "my-container"), o("metadata", `{"owner":"ops"}`)}},
	}
}

// runLeaseParity drives one case against a capture server and returns the
// x-ms-lease-id the action sent (and whether the header was present at all).
func runLeaseParity(t *testing.T, c leaseParityCase, leaseID string) (sent string, present bool) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, present = r.Header["X-Ms-Lease-Id"]
		sent = r.Header.Get("x-ms-lease-id")
		if c.respond != nil {
			c.respond(w)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	inputs := []*core.Connection{
		s("account_name", "devstoreaccount1"),
		{Name: "account_key", Type: core.ConnectionTypeSecret, Value: parityTestKey},
		s("endpoint", srv.URL),
	}
	inputs = append(inputs, c.resource...)
	if leaseID != "" {
		inputs = append(inputs, s("lease_id", leaseID))
	}

	out, err := c.execute(&core.Flow{}, nil, inputs)
	if err != nil {
		t.Fatalf("%s: hard error: %v", c.id, err)
	}
	if out["success"] != true {
		t.Fatalf("%s: soft failure: %v", c.id, out["error"])
	}
	return sent, present
}

// chdirWorkspace gives blob_download somewhere to emit its media file. The
// other cases are indifferent to the cwd.
func chdirWorkspace(t *testing.T) {
	t.Helper()
	ws := t.TempDir()
	if real, err := filepath.EvalSymlinks(ws); err == nil {
		ws = real
	}
	old, _ := os.Getwd()
	if err := os.Chdir(ws); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
}

// TestLeaseIDIsSentWhenSet — the parity claim itself. Every action that offers
// the field must put it on the wire as x-ms-lease-id, or a leased blob stays
// unwritable no matter what the operator types.
func TestLeaseIDIsSentWhenSet(t *testing.T) {
	chdirWorkspace(t)
	for _, c := range leaseParityCases() {
		t.Run(c.id, func(t *testing.T) {
			sent, present := runLeaseParity(t, c, theLeaseID)
			if !present {
				t.Fatalf("%s: no x-ms-lease-id header sent, though lease_id was set", c.id)
			}
			if sent != theLeaseID {
				t.Errorf("%s: x-ms-lease-id = %q, want %q", c.id, sent, theLeaseID)
			}
		})
	}
}

// TestLeaseIDIsAbsentWhenBlank — the sharper half. An empty x-ms-lease-id is
// not "no lease": the service reads it as an invalid one and refuses the call,
// so every ordinary unleased operation would break the moment the header were
// emitted unconditionally.
func TestLeaseIDIsAbsentWhenBlank(t *testing.T) {
	chdirWorkspace(t)
	for _, c := range leaseParityCases() {
		t.Run(c.id, func(t *testing.T) {
			sent, present := runLeaseParity(t, c, "")
			if present {
				t.Errorf("%s: sent x-ms-lease-id: %q with lease_id left blank — an empty lease header is refused, not ignored", c.id, sent)
			}
		})
	}
}

// TestLeaseParityCasesCoverEveryLeaseAction ties this file to the declaration
// table: a lease_id input added to an action with no case here would otherwise
// never be proven to reach the wire.
func TestLeaseParityCasesCoverEveryLeaseAction(t *testing.T) {
	covered := map[string]bool{}
	for _, c := range leaseParityCases() {
		if covered[c.id] {
			t.Errorf("%s: duplicated in leaseParityCases()", c.id)
		}
		covered[c.id] = true
		if !leaseIDActions[c.id] {
			t.Errorf("%s: has a lease-parity case but is not in leaseIDActions", c.id)
		}
	}
	for id := range leaseIDActions {
		if !covered[id] {
			t.Errorf("%s: declares lease_id but no lease-parity case proves it is sent", id)
		}
	}
}
