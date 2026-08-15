package apollo_common

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

// The default path: no reveal asked for, so no email came back. That is a
// one-flag fix, and the message must say so rather than leaving the reader to
// conclude the data is unavailable.
func TestRevealHint_ExplainsUnsetFlag(t *testing.T) {
	RegisterTestingT(t)

	hint := RevealHint(map[string]interface{}{"name": "Ceri Henfrey"}, false)

	Expect(hint).To(ContainSubstring("was NOT set"))
	Expect(hint).To(ContainSubstring("defaults to false"))
	Expect(hint).To(ContainSubstring("1 credit"))
	// Must not blame the plan when the flag simply was not sent.
	Expect(strings.ToLower(hint)).ToNot(ContainSubstring("plan"))
}

// Reveal WAS requested and still nothing came back — only now are "no data" or
// "credits exhausted" the honest explanations.
func TestRevealHint_WhenRevealWasRequested(t *testing.T) {
	RegisterTestingT(t)

	hint := RevealHint(map[string]interface{}{"name": "Ceri Henfrey"}, true)

	Expect(hint).To(ContainSubstring("even though Reveal Personal Emails was set"))
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

// The withheld-data notice must lead with the reveal flag and explicitly deny
// the plan explanation as the default reading. Getting this backwards is what
// caused the integration to be written off as unusable.
func TestGatePrefix_AttributesToRevealFlagNotPlan(t *testing.T) {
	RegisterTestingT(t)

	out := GatePrefix("Found 3 people", []map[string]interface{}{
		{"first_name": "Becky", "last_name_obfuscated": "W***n"},
		{"has_email": true, "email": ""},
		{"first_name": "Ok", "last_name": "Person", "email": "ok@example.com"},
	})

	Expect(out).To(ContainSubstring("USUALLY NOT a plan or credit problem"))
	Expect(out).To(ContainSubstring("People Search is free"))
	Expect(out).To(ContainSubstring("Reveal Personal Emails set to TRUE"))
	// has_email:true is Apollo being accurate, not misleading — say so.
	Expect(out).To(ContainSubstring("an email EXISTS"))
	Expect(out).To(ContainSubstring("2 of 3"))
	Expect(out).To(ContainSubstring("Found 3 people"))
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
