package opentofu_test

import (
	"reflect"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/opentofu"
	applyaction "flomation.app/automate/executor/actions/opentofu/apply"
	destroyaction "flomation.app/automate/executor/actions/opentofu/destroy"
	planaction "flomation.app/automate/executor/actions/opentofu/plan"
)

func findInput(name string, conns []core.Connection) (core.Connection, bool) {
	for _, c := range conns {
		if c.Name == name {
			return c, true
		}
	}
	return core.Connection{}, false
}

// TestSharedInputsDoNotDrift asserts that every action's inline Inputs literal
// reproduces opentofu.SharedInputs verbatim. The inline copies exist only because
// the manifest generator requires an inline composite literal (see SharedInputs'
// doc comment); this test makes SharedInputs the enforced source of truth so a
// copy that drifts — e.g. adding a backend_auth provider to one action but not
// the others — fails CI with the offending field named.
func TestSharedInputsDoNotDrift(t *testing.T) {
	actions := map[string][]core.Connection{
		"plan":    planaction.Inputs[:],
		"apply":   applyaction.Inputs[:],
		"destroy": destroyaction.Inputs[:],
	}

	for _, want := range opentofu.SharedInputs {
		for action, inputs := range actions {
			got, ok := findInput(want.Name, inputs)
			if !ok {
				t.Errorf("%s is missing shared input %q", action, want.Name)
				continue
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s input %q has drifted from opentofu.SharedInputs:\n got=%+v\nwant=%+v", action, want.Name, got, want)
			}
		}
	}
}
