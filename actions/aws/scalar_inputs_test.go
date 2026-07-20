package aws

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestInputBool(t *testing.T) {
	RegisterTestingT(t)

	// A real bool value (the runtime stores booleans as a plain bool).
	Expect(InputBool("x", []*core.Connection{{Name: "x", Type: core.ConnectionTypeBoolean, Value: true}})).To(BeTrue())
	Expect(InputBool("x", []*core.Connection{{Name: "x", Type: core.ConnectionTypeBoolean, Value: false}})).To(BeFalse())
	// String forms that a substituted ${var} might arrive as.
	Expect(InputBool("x", []*core.Connection{conn("x", "true")})).To(BeTrue())
	Expect(InputBool("x", []*core.Connection{conn("x", "YES")})).To(BeTrue())
	Expect(InputBool("x", []*core.Connection{conn("x", "1")})).To(BeTrue())
	// Anything else, or absent, is false.
	Expect(InputBool("x", []*core.Connection{conn("x", "false")})).To(BeFalse())
	Expect(InputBool("x", []*core.Connection{conn("x", "")})).To(BeFalse())
	Expect(InputBool("missing", []*core.Connection{conn("x", "true")})).To(BeFalse())
}

func TestInputInt(t *testing.T) {
	RegisterTestingT(t)

	// Native JSON number (float64), as a wired output arrives.
	n, ok := InputInt("x", []*core.Connection{{Name: "x", Type: core.ConnectionTypeInteger, Value: float64(20)}})
	Expect(ok).To(BeTrue())
	Expect(n).To(Equal(int64(20)))

	// A numeric string (a ${var} substituted into a field).
	n, ok = InputInt("x", []*core.Connection{conn("x", "100")})
	Expect(ok).To(BeTrue())
	Expect(n).To(Equal(int64(100)))

	// Blank, absent, and non-numeric all report "not set" so callers leave the
	// AWS field unset rather than sending a spurious 0.
	_, ok = InputInt("x", []*core.Connection{conn("x", "")})
	Expect(ok).To(BeFalse())
	_, ok = InputInt("x", []*core.Connection{conn("x", "abc")})
	Expect(ok).To(BeFalse())
	_, ok = InputInt("missing", []*core.Connection{conn("x", "5")})
	Expect(ok).To(BeFalse())
}
