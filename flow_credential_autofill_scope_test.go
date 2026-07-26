package core

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
// The documented happy path of the headline feature was a no-op.
//
// It got there because the marker was applied across 178 action files with one
// mechanical edit that swept up the trigger too. A mechanical guard is therefore
// the right shape of defence — reviewing 178 near-identical diffs by eye is how
// it was missed the first time.
//
// If Launch ever learns to resolve credential metadata, delete this test in the
// same change that teaches it — not before.
func TestNoTriggerDeclaresFromCredentialMeta(t *testing.T) {
	raw, err := os.ReadFile("internal/assets/manifest/manifest.json")
	if err != nil {
		t.Skipf("manifest not readable: %v", err)
	}
	var manifest map[string]struct {
		Inputs []struct {
			Name               string `json:"name"`
			FromCredentialMeta string `json:"from_credential_meta"`
		} `json:"inputs"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("cannot parse the manifest: %v", err)
	}
	if len(manifest) == 0 {
		t.Skip("manifest is empty")
	}

	var offenders []string
	linked := 0
	for actionID, entry := range manifest {
		for _, in := range entry.Inputs {
			if in.FromCredentialMeta == "" {
				continue
			}
			linked++
			if strings.HasPrefix(actionID, "trigger/") {
				offenders = append(offenders, actionID+"#"+in.Name)
			}
		}
	}

	if len(offenders) > 0 {
		t.Errorf("%d trigger input(s) declare FromCredentialMeta, which nothing honours — Launch reads the trigger row directly and the input stays blank, so the trigger silently never fires:\n  %s",
			len(offenders), strings.Join(offenders, "\n  "))
	}
	// Guard the guard: if the marker vanishes entirely this test would pass while
	// asserting nothing.
	if linked == 0 {
		t.Error("no input anywhere declares FromCredentialMeta — this test is no longer checking anything")
	}
}

// A required input must not also claim it will be filled in for the operator:
// Required means the editor blocks saving until it is set, so the auto-fill can
// never be the thing that satisfies it.
func TestNoRequiredInputClaimsToBeAutoFilled(t *testing.T) {
	raw, err := os.ReadFile("internal/assets/manifest/manifest.json")
	if err != nil {
		t.Skipf("manifest not readable: %v", err)
	}
	var manifest map[string]struct {
		Inputs []struct {
			Name               string `json:"name"`
			Required           bool   `json:"required"`
			FromCredentialMeta string `json:"from_credential_meta"`
		} `json:"inputs"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("cannot parse the manifest: %v", err)
	}

	var bad []string
	for actionID, entry := range manifest {
		for _, in := range entry.Inputs {
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
