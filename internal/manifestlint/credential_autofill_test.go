package manifestlint

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// FromCredentialMeta is EXECUTOR-ONLY. autofillFromCredential runs inside
// executeNodeActionOnly, which triggers never reach: Launch polls the trigger row
// directly and reads its saved config, so nothing fills a blank input.
//
// This shipped as a 🔴 defect. The Salesforce poll trigger carried the marker,
// was not Required, and its placeholder read "Leave blank — taken from your
// connection". An operator who did as told got a trigger that silently never
// fired: normaliseInstanceURL("") errors, checkTrigger logs a Warn and returns,
// the flow saves 201, the trigger row exists, and the editor shows it enabled.
// The documented happy path of that release's headline capability was a no-op.
//
// It got there because the marker was applied across 178 action files in one
// mechanical edit that swept up the trigger too. A mechanical guard is therefore
// the right shape of defence: reviewing 178 near-identical diffs by eye is how it
// was missed, and uniformity is exactly what hides the one file that should
// differ.
//
// If Launch ever learns to resolve credential metadata, delete this test in the
// same change that teaches it — not before.
func TestNoTriggerDeclaresFromCredentialMeta(t *testing.T) {
	linked, byAction := credentialAutofillInputs(t)

	var offenders []string
	for actionID, inputs := range byAction {
		if !strings.HasPrefix(actionID, "trigger/") {
			continue
		}
		for _, name := range inputs {
			offenders = append(offenders, actionID+"#"+name)
		}
	}

	if len(offenders) > 0 {
		t.Errorf("%d trigger input(s) declare FromCredentialMeta, which nothing honours — Launch reads the trigger row directly and the input stays blank, so the trigger silently never fires:\n  %s",
			len(offenders), strings.Join(offenders, "\n  "))
	}
	// Guard the guard: if the marker vanished entirely this would pass while
	// asserting nothing.
	if linked == 0 {
		t.Error("no input anywhere declares FromCredentialMeta — this test is no longer checking anything")
	}
}

// Required means the editor blocks saving until the operator sets it, so an
// auto-fill can never be the thing that satisfies it. Declaring both is a
// contradiction, and the operator-visible symptom is a field the UI insists on
// while the placeholder says to leave it alone.
func TestNoRequiredInputClaimsToBeAutoFilled(t *testing.T) {
	raw, err := os.ReadFile(manifestRelPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest map[string]struct {
		Inputs []struct {
			Name               string `json:"name"`
			Required           bool   `json:"required"`
			FromCredentialMeta string `json:"from_credential_meta"`
		} `json:"inputs"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}

	var bad []string
	for actionID, e := range manifest {
		for _, in := range e.Inputs {
			if in.FromCredentialMeta != "" && in.Required {
				bad = append(bad, actionID+"#"+in.Name)
			}
		}
	}
	if len(bad) > 0 {
		t.Errorf("%d input(s) are both Required and auto-filled, which contradict each other:\n  %s",
			len(bad), strings.Join(bad, "\n  "))
	}
}

// credentialAutofillInputs returns the total count of inputs declaring
// FromCredentialMeta and a map of actionID -> input names, read from the
// committed manifest rather than the Go structs: the manifest is the artefact
// that actually ships and that the executor reads at run time.
func credentialAutofillInputs(t *testing.T) (int, map[string][]string) {
	t.Helper()
	raw, err := os.ReadFile(manifestRelPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest map[string]struct {
		Inputs []struct {
			Name               string `json:"name"`
			FromCredentialMeta string `json:"from_credential_meta"`
		} `json:"inputs"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	if len(manifest) == 0 {
		t.Fatal("manifest is empty — this test would pass vacuously")
	}

	total := 0
	out := map[string][]string{}
	for actionID, e := range manifest {
		for _, in := range e.Inputs {
			if in.FromCredentialMeta == "" {
				continue
			}
			total++
			out[actionID] = append(out[actionID], in.Name)
		}
	}
	return total, out
}
