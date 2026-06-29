package core

import (
	"testing"

	. "github.com/onsi/gomega"
)

// ─────────────────────────────────────────────────────────────────
// ParseReference
// ─────────────────────────────────────────────────────────────────

func TestParseReference_NamespaceShortForm(t *testing.T) {
	RegisterTestingT(t)
	ref, ok := ParseReference("flow.user_name")
	Expect(ok).To(BeTrue())
	Expect(ref.Namespace).To(Equal("flow"))
	Expect(ref.Root).To(Equal("user_name"))
	Expect(ref.Child).To(Equal(""))
	Expect(ref.Path).To(BeEmpty())
}

func TestParseReference_NamespaceWithChild(t *testing.T) {
	RegisterTestingT(t)
	ref, ok := ParseReference("flow.user.name")
	Expect(ok).To(BeTrue())
	Expect(ref.Namespace).To(Equal("flow"))
	Expect(ref.Root).To(Equal("user"))
	Expect(ref.Child).To(Equal("name"))
	Expect(ref.Path).To(BeEmpty())
}

func TestParseReference_NamespaceWithPath(t *testing.T) {
	RegisterTestingT(t)
	ref, ok := ParseReference("flow.user.profile.email")
	Expect(ok).To(BeTrue())
	Expect(ref.Namespace).To(Equal("flow"))
	Expect(ref.Root).To(Equal("user"))
	Expect(ref.Child).To(Equal("profile"))
	Expect(ref.Path).To(Equal([]string{"email"}))
}

func TestParseReference_ParentOutputShortForm(t *testing.T) {
	// Unprefixed: parent-node-output reference. Root = node id,
	// Child = output key.
	RegisterTestingT(t)
	ref, ok := ParseReference("node_abc.body")
	Expect(ok).To(BeTrue())
	Expect(ref.Namespace).To(Equal(""))
	Expect(ref.Root).To(Equal("node_abc"))
	Expect(ref.Child).To(Equal("body"))
	Expect(ref.Path).To(BeEmpty())
}

func TestParseReference_ParentOutputWithPath(t *testing.T) {
	RegisterTestingT(t)
	ref, ok := ParseReference("node_abc.body.user.name")
	Expect(ok).To(BeTrue())
	Expect(ref.Namespace).To(Equal(""))
	Expect(ref.Root).To(Equal("node_abc"))
	Expect(ref.Child).To(Equal("body"))
	Expect(ref.Path).To(Equal([]string{"user", "name"}))
}

func TestParseReference_BracketIndex(t *testing.T) {
	RegisterTestingT(t)
	ref, ok := ParseReference("node_abc.items[3]")
	Expect(ok).To(BeTrue())
	Expect(ref.Namespace).To(Equal(""))
	Expect(ref.Root).To(Equal("node_abc"))
	Expect(ref.Child).To(Equal("items"))
	Expect(ref.Path).To(Equal([]string{"3"}))
}

func TestParseReference_MixedDotAndBracket(t *testing.T) {
	// The expected workhorse: API response → path into items
	// → array index → field on the indexed item.
	RegisterTestingT(t)
	ref, ok := ParseReference("node_abc.body.items[0].title")
	Expect(ok).To(BeTrue())
	Expect(ref.Namespace).To(Equal(""))
	Expect(ref.Root).To(Equal("node_abc"))
	Expect(ref.Child).To(Equal("body"))
	Expect(ref.Path).To(Equal([]string{"items", "0", "title"}))
}

func TestParseReference_DeepBracketChain(t *testing.T) {
	RegisterTestingT(t)
	ref, ok := ParseReference("node_abc.data[0][1]")
	Expect(ok).To(BeTrue())
	Expect(ref.Child).To(Equal("data"))
	Expect(ref.Path).To(Equal([]string{"0", "1"}))
}

func TestParseReference_Empty_NotOk(t *testing.T) {
	RegisterTestingT(t)
	_, ok := ParseReference("")
	Expect(ok).To(BeFalse())
}

func TestParseReference_NoSeparator_NotOk(t *testing.T) {
	// A bare token with no dot/bracket isn't a meaningful reference.
	RegisterTestingT(t)
	_, ok := ParseReference("just_a_name")
	Expect(ok).To(BeFalse())
}

func TestParseReference_AllRecognisedNamespaces(t *testing.T) {
	// Pins the closed set so we notice if a namespace is removed
	// (would break ParseReference's classification).
	RegisterTestingT(t)
	for _, ns := range []string{"secrets", "secret", "credentials", "credential", "env", "flow", "var", "user", "loop", "trigger", "input"} {
		ref, ok := ParseReference(ns + ".something")
		Expect(ok).To(BeTrue(), ns)
		Expect(ref.Namespace).To(Equal(ns), ns)
		Expect(ref.Root).To(Equal("something"), ns)
	}
}

// ─────────────────────────────────────────────────────────────────
// ResolvePath
// ─────────────────────────────────────────────────────────────────

func TestResolvePath_EmptyPathReturnsInputUnchanged(t *testing.T) {
	// Defensive: existing callers that always supply a (possibly
	// empty) path should get back the input value unchanged. This
	// is the property that preserves backward-compatibility for
	// every existing ${ns.x} → value lookup.
	RegisterTestingT(t)
	v := map[string]interface{}{"a": 1}
	out, err := ResolvePath(v, nil)
	Expect(err).NotTo(HaveOccurred())
	Expect(out).To(Equal(v))
}

func TestResolvePath_SingleMapField(t *testing.T) {
	RegisterTestingT(t)
	out, err := ResolvePath(
		map[string]interface{}{"user": map[string]interface{}{"name": "Andy"}},
		[]string{"user", "name"},
	)
	Expect(err).NotTo(HaveOccurred())
	Expect(out).To(Equal("Andy"))
}

func TestResolvePath_SliceIndex(t *testing.T) {
	RegisterTestingT(t)
	out, err := ResolvePath(
		[]interface{}{"a", "b", "c"},
		[]string{"1"},
	)
	Expect(err).NotTo(HaveOccurred())
	Expect(out).To(Equal("b"))
}

func TestResolvePath_MapThenSliceThenMap(t *testing.T) {
	RegisterTestingT(t)
	out, err := ResolvePath(
		map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{"id": 100, "title": "first"},
				map[string]interface{}{"id": 200, "title": "second"},
			},
		},
		[]string{"items", "1", "title"},
	)
	Expect(err).NotTo(HaveOccurred())
	Expect(out).To(Equal("second"))
}

func TestResolvePath_JSONStringRootIsParsed(t *testing.T) {
	// The Web/HTTP case: response_body is a raw JSON string, the
	// resolver auto-parses it before walking the path. Without
	// this, every Web action would need an intermediate JSON-parse
	// node.
	RegisterTestingT(t)
	jsonStr := `{"user":{"id":42,"name":"Andy"}}`
	out, err := ResolvePath(jsonStr, []string{"user", "name"})
	Expect(err).NotTo(HaveOccurred())
	Expect(out).To(Equal("Andy"))
}

func TestResolvePath_JSONStringWithArray(t *testing.T) {
	RegisterTestingT(t)
	jsonStr := `{"items":[{"id":1},{"id":2},{"id":3}]}`
	out, err := ResolvePath(jsonStr, []string{"items", "0", "id"})
	Expect(err).NotTo(HaveOccurred())
	// JSON numbers parse as float64 in Go's default encoder.
	Expect(out).To(BeEquivalentTo(1))
}

func TestResolvePath_MissingMapKey_Errors(t *testing.T) {
	RegisterTestingT(t)
	_, err := ResolvePath(
		map[string]interface{}{"a": 1},
		[]string{"b"},
	)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("b"))
}

func TestResolvePath_OutOfBoundsIndex_Errors(t *testing.T) {
	RegisterTestingT(t)
	_, err := ResolvePath(
		[]interface{}{"a", "b"},
		[]string{"5"},
	)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("out of bounds"))
}

func TestResolvePath_NonNumericIndexOnSlice_Errors(t *testing.T) {
	RegisterTestingT(t)
	_, err := ResolvePath(
		[]interface{}{"a", "b"},
		[]string{"foo"},
	)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("numeric"))
}

func TestResolvePath_DescentIntoScalar_Errors(t *testing.T) {
	// User typed too deep — the value is a scalar at this point
	// and can't be descended further. Should surface a clear
	// error, not a silent empty result.
	RegisterTestingT(t)
	_, err := ResolvePath(
		map[string]interface{}{"name": "Andy"},
		[]string{"name", "first"},
	)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("can't descend"))
}

func TestResolvePath_NonJSONStringRoot_Errors(t *testing.T) {
	// A string that isn't JSON should fail clearly when path
	// resolution is attempted on it.
	RegisterTestingT(t)
	_, err := ResolvePath("just a regular string", []string{"foo"})
	Expect(err).To(HaveOccurred())
}

// Integration: ParseReference + ResolvePath end-to-end on a realistic
// Web response shape. Captures the "what the user actually types"
// scenario.
func TestParseAndResolve_RealisticWebResponse(t *testing.T) {
	RegisterTestingT(t)
	ref, ok := ParseReference("web_node.response_body.data.users[0].email")
	Expect(ok).To(BeTrue())
	Expect(ref.Root).To(Equal("web_node"))
	Expect(ref.Child).To(Equal("response_body"))
	Expect(ref.Path).To(Equal([]string{"data", "users", "0", "email"}))

	// The Web action's response_body is a JSON string at runtime.
	responseBody := `{"data":{"users":[{"email":"a@example.com"},{"email":"b@example.com"}]}}`
	// In the real substituteVariables call: look up child on the
	// parent node's outputs, then walk the path.
	val, err := ResolvePath(responseBody, ref.Path)
	Expect(err).NotTo(HaveOccurred())
	Expect(val).To(Equal("a@example.com"))
}
