package apollo_common

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

func TestIsGatedRecord(t *testing.T) {
	RegisterTestingT(t)

	// Obfuscated surname → gated.
	Expect(IsGatedRecord(map[string]interface{}{
		"first_name": "Brian", "last_name_obfuscated": "Mc***y",
	})).To(BeTrue())

	// has_email true (bool) but no email value → gated.
	Expect(IsGatedRecord(map[string]interface{}{
		"first_name": "Jo", "has_email": true,
	})).To(BeTrue())

	// has_city true (as the string "Yes") but no city → gated.
	Expect(IsGatedRecord(map[string]interface{}{
		"has_city": "Yes",
	})).To(BeTrue())

	// Fully revealed record → not gated.
	Expect(IsGatedRecord(map[string]interface{}{
		"first_name": "Ada", "last_name": "Lovelace",
		"email": "ada@example.com", "has_email": true, "city": "London", "has_city": true,
	})).To(BeFalse())

	Expect(IsGatedRecord(nil)).To(BeFalse())
}

func TestGatePrefix(t *testing.T) {
	RegisterTestingT(t)

	gated := []map[string]interface{}{
		{"first_name": "Brian", "last_name_obfuscated": "Mc***y"},
		{"first_name": "Ada", "last_name": "Lovelace", "email": "ada@x.com"},
	}
	out := GatePrefix("Found 2 people", gated)
	Expect(out).To(HavePrefix("NOTE - PERSONAL DATA WITHHELD"))
	Expect(out).To(ContainSubstring("1 of 2 record(s)"))
	// The cause is the un-set reveal flag, NOT the plan — see reveal_test.go.
	Expect(out).To(ContainSubstring("Reveal Personal Emails ON by default"))
	Expect(strings.HasSuffix(out, "Found 2 people")).To(BeTrue())

	// No gated records → summary unchanged (no false alarm on real data).
	clean := []map[string]interface{}{{"first_name": "Ada", "last_name": "Lovelace", "email": "ada@x.com"}}
	Expect(GatePrefix("Found 1 people", clean)).To(Equal("Found 1 people"))
}
