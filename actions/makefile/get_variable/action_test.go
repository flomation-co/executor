package get_variable

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestParseVariable_SimpleAssignment(t *testing.T) {
	RegisterTestingT(t)

	output := `# makefile database
CC = gcc
CFLAGS = -Wall -O2
LDFLAGS = -lm
`
	value, found := parseVariable(output, "CC")
	Expect(found).To(BeTrue())
	Expect(value).To(Equal("gcc"))

	value, found = parseVariable(output, "CFLAGS")
	Expect(found).To(BeTrue())
	Expect(value).To(Equal("-Wall -O2"))
}

func TestParseVariable_ImmediateAssignment(t *testing.T) {
	RegisterTestingT(t)

	output := `VERSION := 1.2.3
`
	value, found := parseVariable(output, "VERSION")
	Expect(found).To(BeTrue())
	Expect(value).To(Equal("1.2.3"))
}

func TestParseVariable_ConditionalAssignment(t *testing.T) {
	RegisterTestingT(t)

	output := `PREFIX ?= /usr/local
`
	value, found := parseVariable(output, "PREFIX")
	Expect(found).To(BeTrue())
	Expect(value).To(Equal("/usr/local"))
}

func TestParseVariable_AppendAssignment(t *testing.T) {
	RegisterTestingT(t)

	output := `SOURCES += main.c utils.c
`
	value, found := parseVariable(output, "SOURCES")
	Expect(found).To(BeTrue())
	Expect(value).To(Equal("main.c utils.c"))
}

func TestParseVariable_NotFound(t *testing.T) {
	RegisterTestingT(t)

	output := `CC = gcc
`
	_, found := parseVariable(output, "NONEXISTENT")
	Expect(found).To(BeFalse())
}

func TestParseVariable_EmptyValue(t *testing.T) {
	RegisterTestingT(t)

	output := `EMPTY =
`
	value, found := parseVariable(output, "EMPTY")
	Expect(found).To(BeTrue())
	Expect(value).To(Equal(""))
}

func TestParseVariable_PartialNameMatch(t *testing.T) {
	RegisterTestingT(t)

	output := `CC = gcc
CCOPTS = -O2
`
	value, found := parseVariable(output, "CC")
	Expect(found).To(BeTrue())
	Expect(value).To(Equal("gcc"))
}
