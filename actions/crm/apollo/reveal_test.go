package apollo_common

import (
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// Reveal deliberately switched off: no email came back, and the message must
// name the switch rather than leave the reader concluding the data is
// unavailable.
func TestRevealHint_ExplainsSwitchedOffFlag(t *testing.T) {
	RegisterTestingT(t)

	hint := RevealHint(map[string]interface{}{"name": "Ceri Henfrey"}, false)

	Expect(hint).To(ContainSubstring("switched OFF"))
	Expect(hint).To(ContainSubstring("1 credit"))
	// Must not blame the plan when reveal simply was not requested.
	Expect(strings.ToLower(hint)).ToNot(ContainSubstring("plan"))
}

// Reveal WAS requested and still nothing came back — only now are "no data" or
// "credits exhausted" the honest explanations.
func TestRevealHint_WhenRevealWasRequested(t *testing.T) {
	RegisterTestingT(t)

	hint := RevealHint(map[string]interface{}{"name": "Ceri Henfrey"}, true)

	Expect(hint).To(ContainSubstring("even though Reveal Personal Emails was on"))
	Expect(hint).To(ContainSubstring("no personal email on file"))
	Expect(hint).To(ContainSubstring("credits"))
}

// An email present means there is nothing to explain.
func TestRevealHint_SilentWhenEmailPresent(t *testing.T) {
	RegisterTestingT(t)

	Expect(RevealHint(map[string]interface{}{"email": "ceri@example.com"}, false)).To(Equal(""))
	Expect(RevealHint(map[string]interface{}{"email": "ceri@example.com"}, true)).To(Equal(""))
	Expect(RevealHint(nil, false)).To(Equal(""))
}

// The ENRICHMENT notice must attribute a null email to the reveal flag, not the
// plan. Getting this backwards is what caused the integration to be written off
// as unusable. (The SEARCH variant carries different, search-specific advice —
// see TestGatePrefixSearch_GivesSearchSpecificAdvice.)
func TestGatePrefix_AttributesToRevealFlagNotPlan(t *testing.T) {
	RegisterTestingT(t)

	out := GatePrefix("Enriched 3 people", []map[string]interface{}{
		{"first_name": "Becky", "last_name_obfuscated": "W***n"},
		{"has_email": true, "email": ""},
		{"first_name": "Ok", "last_name": "Person", "email": "ok@example.com"},
	})

	Expect(out).To(ContainSubstring("USUALLY the reveal flag rather than the plan"))
	Expect(out).To(ContainSubstring("Reveal Personal Emails ON by default"))
	Expect(out).To(ContainSubstring("2 of 3"))
	Expect(out).To(ContainSubstring("Enriched 3 people"))
}

func TestGatePrefix_SilentWhenNothingWithheld(t *testing.T) {
	RegisterTestingT(t)

	out := GatePrefix("Found 1 people", []map[string]interface{}{
		{"first_name": "Ok", "last_name": "Person", "email": "ok@example.com"},
	})
	Expect(out).To(Equal("Found 1 people"))
}

func TestBoolValue(t *testing.T) {
	RegisterTestingT(t)

	Expect(BoolValue("missing", qInputs())).To(BeFalse())
}

// The default-ON behaviour rests entirely on three states being
// distinguishable: never configured, explicitly on, explicitly off. If an
// untouched input were indistinguishable from an explicit false, either the
// default could never apply or an author's decision to switch reveal off would
// be overridden — and the second would spend their credits against their wishes.
func TestBoolValueDefault_TriState(t *testing.T) {
	RegisterTestingT(t)

	boolInput := func(v interface{}) []*core.Connection {
		return []*core.Connection{{Name: "reveal", Type: core.ConnectionTypeBoolean, Value: v}}
	}

	// Never configured → the default applies.
	Expect(BoolValueDefault("reveal", qInputs(), true)).To(BeTrue())
	Expect(BoolValueDefault("reveal", boolInput(nil), true)).To(BeTrue())

	// Explicitly off → honoured, NOT overridden by the default.
	Expect(BoolValueDefault("reveal", boolInput(false), true)).To(BeFalse())

	// Explicitly on → on.
	Expect(BoolValueDefault("reveal", boolInput(true), false)).To(BeTrue())

	// A ${...} substitution arrives as a string and must still be honoured.
	Expect(BoolValueDefault("reveal", boolInput("false"), true)).To(BeFalse())
	Expect(BoolValueDefault("reveal", boolInput("true"), false)).To(BeTrue())
}
