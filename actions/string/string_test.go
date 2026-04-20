package string_common

import (
	"testing"

	core "flomation.app/automate/executor"
	string_concatenate "flomation.app/automate/executor/actions/string/concatenate"
	string_contains "flomation.app/automate/executor/actions/string/contains"
	string_join "flomation.app/automate/executor/actions/string/join"
	string_lower_case "flomation.app/automate/executor/actions/string/lower_case"
	string_repeat "flomation.app/automate/executor/actions/string/repeat"
	string_replace "flomation.app/automate/executor/actions/string/replace"
	string_substring "flomation.app/automate/executor/actions/string/substring"
	string_trim "flomation.app/automate/executor/actions/string/trim"
	string_trim_end "flomation.app/automate/executor/actions/string/trim_end"
	string_trim_start "flomation.app/automate/executor/actions/string/trim_start"
	string_upper_case "flomation.app/automate/executor/actions/string/upper_case"

	. "github.com/onsi/gomega"
)

func conn(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

func numConn(name string, value int64) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeInteger, Value: value}
}

func TestSubstring(t *testing.T) {
	RegisterTestingT(t)
	out, err := string_substring.Execute(nil, nil, []*core.Connection{
		conn("value", "Hello World"), numConn("start", 6), numConn("end", 11),
	})
	Expect(err).To(BeNil())
	Expect(out["result"]).To(Equal("World"))
}

func TestReplace(t *testing.T) {
	RegisterTestingT(t)
	out, err := string_replace.Execute(nil, nil, []*core.Connection{
		conn("value", "foo bar foo"), conn("search", "foo"), conn("replace", "baz"),
	})
	Expect(err).To(BeNil())
	Expect(out["result"]).To(Equal("baz bar baz"))
	Expect(out["count"]).To(Equal(2))
}

func TestContains(t *testing.T) {
	RegisterTestingT(t)
	out, err := string_contains.Execute(nil, nil, []*core.Connection{
		conn("value", "Hello World"), conn("search", "World"),
	})
	Expect(err).To(BeNil())
	Expect(out["result"]).To(BeTrue())

	out, err = string_contains.Execute(nil, nil, []*core.Connection{
		conn("value", "Hello World"), conn("search", "xyz"),
	})
	Expect(err).To(BeNil())
	Expect(out["result"]).To(BeFalse())
}

func TestJoin(t *testing.T) {
	RegisterTestingT(t)
	out, err := string_join.Execute(nil, nil, []*core.Connection{
		{Name: "array", Type: core.ConnectionTypeObject, Value: []interface{}{"a", "b", "c"}},
		conn("separator", "-"),
	})
	Expect(err).To(BeNil())
	Expect(out["result"]).To(Equal("a-b-c"))
}

func TestRepeat(t *testing.T) {
	RegisterTestingT(t)
	out, err := string_repeat.Execute(nil, nil, []*core.Connection{
		conn("value", "ab"), numConn("count", 3),
	})
	Expect(err).To(BeNil())
	Expect(out["result"]).To(Equal("ababab"))
}

func TestConcatenate(t *testing.T) {
	RegisterTestingT(t)
	out, err := string_concatenate.Execute(nil, nil, []*core.Connection{
		conn("a", "Hello"), conn("b", " "), conn("c", "World"),
	})
	Expect(err).To(BeNil())
	Expect(out["result"]).To(Equal("Hello World"))
}

func TestUpperCase(t *testing.T) {
	RegisterTestingT(t)
	out, err := string_upper_case.Execute(nil, nil, []*core.Connection{
		conn("value", "hello world"),
	})
	Expect(err).To(BeNil())
	Expect(out["result"]).To(Equal("HELLO WORLD"))
}

func TestLowerCase(t *testing.T) {
	RegisterTestingT(t)
	out, err := string_lower_case.Execute(nil, nil, []*core.Connection{
		conn("value", "HELLO WORLD"),
	})
	Expect(err).To(BeNil())
	Expect(out["result"]).To(Equal("hello world"))
}

func TestTrim(t *testing.T) {
	RegisterTestingT(t)
	out, err := string_trim.Execute(nil, nil, []*core.Connection{
		conn("value", "  hello  "),
	})
	Expect(err).To(BeNil())
	Expect(out["result"]).To(Equal("hello"))
}

func TestTrimStart(t *testing.T) {
	RegisterTestingT(t)
	out, err := string_trim_start.Execute(nil, nil, []*core.Connection{
		conn("value", "  hello  "),
	})
	Expect(err).To(BeNil())
	Expect(out["result"]).To(Equal("hello  "))
}

func TestTrimEnd(t *testing.T) {
	RegisterTestingT(t)
	out, err := string_trim_end.Execute(nil, nil, []*core.Connection{
		conn("value", "  hello  "),
	})
	Expect(err).To(BeNil())
	Expect(out["result"]).To(Equal("  hello"))
}
