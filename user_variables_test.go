package core

import (
	"testing"

	. "github.com/onsi/gomega"
)

// Smoke tests for the ${user.X} substitution namespace via ExecutionContext.
// Concrete substitution wiring is exercised end-to-end through Flow.Execute
// in the broader test suite; these tests focus on the lookup itself.

func TestExecutionContext_UserVariables_LookupPopulated(t *testing.T) {
	RegisterTestingT(t)

	ctx := &ExecutionContext{
		UserVariables: map[string]string{
			"first_name":   "Andy",
			"last_name":    "Esser",
			"full_name":    "Mr Andy Esser",
			"full_address": "10 Downing St\nLondon\nSW1A 2AA",
		},
	}

	Expect(ctx.UserVariables["first_name"]).To(Equal("Andy"))
	Expect(ctx.UserVariables["full_name"]).To(Equal("Mr Andy Esser"))
	Expect(ctx.UserVariables["full_address"]).To(ContainSubstring("\n"))
}

func TestExecutionContext_UserVariables_MissingFieldsAreEmpty(t *testing.T) {
	RegisterTestingT(t)

	ctx := &ExecutionContext{
		UserVariables: map[string]string{
			"first_name": "Andy",
		},
	}

	// Lookup on a missing key returns zero value — empty string. The
	// substitution path treats that as "" and replaces the placeholder,
	// matching ${flow.X} semantics rather than leaving the literal
	// ${user.X} in the output.
	Expect(ctx.UserVariables["last_name"]).To(Equal(""))
	Expect(ctx.UserVariables["full_address"]).To(Equal(""))
}

func TestExecutionContext_UserVariables_NilMapSafe(t *testing.T) {
	RegisterTestingT(t)

	ctx := &ExecutionContext{} // UserVariables is nil
	// Reading from a nil map returns zero value in Go — should not panic.
	Expect(func() { _ = ctx.UserVariables["first_name"] }).ToNot(Panic())
}
