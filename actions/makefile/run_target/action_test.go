package run_target

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestSplitVariables_Simple(t *testing.T) {
	RegisterTestingT(t)

	result := splitVariables("CC=gcc CFLAGS=-O2")
	Expect(result).To(Equal([]string{"CC=gcc", "CFLAGS=-O2"}))
}

func TestSplitVariables_Quoted(t *testing.T) {
	RegisterTestingT(t)

	result := splitVariables(`CC=gcc CFLAGS="-Wall -O2" LDFLAGS=-lm`)
	Expect(result).To(Equal([]string{"CC=gcc", `CFLAGS="-Wall -O2"`, "LDFLAGS=-lm"}))
}

func TestSplitVariables_Empty(t *testing.T) {
	RegisterTestingT(t)

	result := splitVariables("")
	Expect(result).To(BeNil())
}

func TestSplitVariables_ExtraSpaces(t *testing.T) {
	RegisterTestingT(t)

	result := splitVariables("  A=1   B=2  ")
	Expect(result).To(Equal([]string{"A=1", "B=2"}))
}

func TestTruncate(t *testing.T) {
	RegisterTestingT(t)

	Expect(truncate("short", 100)).To(Equal("short"))
	Expect(truncate("long string", 4)).To(Equal("long\n... (output truncated)"))
}

func TestFirstLine(t *testing.T) {
	RegisterTestingT(t)

	Expect(firstLine("first\nsecond\nthird")).To(Equal("first"))
	Expect(firstLine("only")).To(Equal("only"))
	Expect(firstLine("  padded  \n  second  ")).To(Equal("padded"))
}
