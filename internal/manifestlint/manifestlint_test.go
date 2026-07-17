// Package manifestlint validates the committed manifest itself, the way
// iconlint validates the icons in it.
//
// It exists because of a real failure. actions/devops/azuredevops/common.go
// named its shared request helper `func Execute(flow, a Auth, r Request)`.
// The generator treats ANY package-level `func Execute` as an action
// (cmd/manifest/manifest.go, HasExecute), so it invented a phantom action
// "devops/azuredevops" with an empty name AND emitted
//
//	"devops/azuredevops": devops_azuredevops.Execute,
//
// into actions_generated.go — where the helper's signature is not a
// core.Action, so the whole executor stopped compiling. The clean siblings
// (azure/storage, azure/entra) call their helper Do / ExecuteAPI, which is why
// this had never happened before.
//
// Two things made it nastier than it needed to be: the breakage only appears
// AFTER `make manifest` regenerates, so a `go build` run before that step
// reports success; and the phantom is otherwise invisible — a silently extra
// action in the palette with no name.
package manifestlint

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
)

const manifestRelPath = "../assets/manifest/manifest.json"

type entry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        int    `json:"type"`
	Hash        string `json:"hash"`
}

func load(t *testing.T) map[string]entry {
	t.Helper()
	b, err := os.ReadFile(manifestRelPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m map[string]entry
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if len(m) == 0 {
		t.Fatal("manifest is empty — this test would pass vacuously")
	}
	return m
}

// TestNoPhantomActions is the guard for the failure described above. A package
// that only holds shared helpers must not appear in the manifest at all; if it
// does, the giveaway is an empty Name, because there is no action metadata to
// harvest.
func TestNoPhantomActions(t *testing.T) {
	m := load(t)
	var phantom []string
	for id, e := range m {
		if strings.TrimSpace(e.Name) == "" {
			phantom = append(phantom, id)
		}
	}
	sort.Strings(phantom)
	if len(phantom) > 0 {
		t.Errorf(`%d action(s) in the manifest have no name: %v

This almost always means a shared package declares a package-level
"func Execute" that is NOT an action — the generator treats any Execute as one
(cmd/manifest/manifest.go), invents an action for the package, and registers the
helper in actions_generated.go where its signature is not a core.Action, which
breaks the build.

Rename the helper (the convention elsewhere is Do, or ExecuteAPI) and re-run
make manifest.`, len(phantom), phantom)
	}
}

// TestEveryActionHasAHash — the hash is what tells the api an action changed.
// A missing one means the api will never re-ingest that action's inputs.
func TestEveryActionHasAHash(t *testing.T) {
	m := load(t)
	var missing []string
	for id, e := range m {
		if strings.TrimSpace(e.Hash) == "" {
			missing = append(missing, id)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("%d action(s) have no hash: %v", len(missing), missing)
	}
}

// TestActionIDsAreWellFormed — the palette resolves a category from the id by
// splitting on "/" and reading only the first two segments, so an id with no
// slash has no category and lands in the synthetic "Other" group.
func TestActionIDsAreWellFormed(t *testing.T) {
	m := load(t)
	var bad []string
	for id := range m {
		switch {
		case id != strings.TrimSpace(id),
			strings.Contains(id, " "),
			strings.HasPrefix(id, "/"), strings.HasSuffix(id, "/"),
			strings.Contains(id, "//"),
			!strings.Contains(id, "/"):
			bad = append(bad, id)
		}
	}
	sort.Strings(bad)
	if len(bad) > 0 {
		t.Errorf("%d malformed action id(s): %v", len(bad), bad)
	}
}
