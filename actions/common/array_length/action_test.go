package array_length

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

// TestExecute_NativeArray — the most direct case: array_length
// receives a Go slice from an upstream action's typed output.
func TestExecute_NativeArray(t *testing.T) {
	RegisterTestingT(t)
	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "array", Type: core.ConnectionTypeObject, Value: []interface{}{"a", "b", "c"}},
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(result["success"]).To(BeTrue())
	Expect(result["length"]).To(Equal(3))
}

// TestExecute_JSONString — the Web/HTTP response shape. The action
// receives the raw response_body string, parses it as JSON, returns
// the length.
func TestExecute_JSONString(t *testing.T) {
	RegisterTestingT(t)
	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "array", Type: core.ConnectionTypeObject, Value: `[1,2,3,4]`},
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(result["success"]).To(BeTrue())
	Expect(result["length"]).To(Equal(4))
}

// TestExecute_PathIntoObject — the new affordance. Web API returns
// {"items":[...]} and the user sets path="items" to count inside.
func TestExecute_PathIntoObject(t *testing.T) {
	RegisterTestingT(t)
	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "array", Type: core.ConnectionTypeObject, Value: `{"items":[{"id":1},{"id":2},{"id":3}],"total":3}`},
		{Name: "path", Type: core.ConnectionTypeString, Value: "items"},
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(result["success"]).To(BeTrue())
	Expect(result["length"]).To(Equal(3))
}

// TestExecute_DottedPath — nested object paths like "data.results".
func TestExecute_DottedPath(t *testing.T) {
	RegisterTestingT(t)
	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "array", Type: core.ConnectionTypeObject, Value: `{"data":{"results":[10,20,30,40,50]}}`},
		{Name: "path", Type: core.ConnectionTypeString, Value: "data.results"},
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(result["success"]).To(BeTrue())
	Expect(result["length"]).To(Equal(5))
}

// TestExecute_PathWithArrayIndex — numeric segments descend into
// arrays. Useful for shapes like {"pages":[{"items":[...]},...]}
// where the user wants pages[0].items.
func TestExecute_PathWithArrayIndex(t *testing.T) {
	RegisterTestingT(t)
	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "array", Type: core.ConnectionTypeObject, Value: `{"pages":[{"items":["a","b"]},{"items":["c","d","e"]}]}`},
		{Name: "path", Type: core.ConnectionTypeString, Value: "pages.1.items"},
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(result["success"]).To(BeTrue())
	Expect(result["length"]).To(Equal(3))
}

// TestExecute_PathNotFound — clear error surfaced when the path
// doesn't resolve. Important: the error names the failing segment
// so users can debug "did I mistype the path?".
func TestExecute_PathNotFound(t *testing.T) {
	RegisterTestingT(t)
	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "array", Type: core.ConnectionTypeObject, Value: `{"items":[1,2,3]}`},
		{Name: "path", Type: core.ConnectionTypeString, Value: "data"},
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(result["success"]).To(BeFalse())
	Expect(result["error"]).To(ContainSubstring("data"))
}

// TestExecute_PathLeadsToNonArray — path resolves but the target
// isn't an array. Common mistake: path points at a scalar rather
// than the wrapping array.
func TestExecute_PathLeadsToNonArray(t *testing.T) {
	RegisterTestingT(t)
	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "array", Type: core.ConnectionTypeObject, Value: `{"total":42,"items":[1,2,3]}`},
		{Name: "path", Type: core.ConnectionTypeString, Value: "total"},
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(result["success"]).To(BeFalse())
	Expect(result["error"]).To(ContainSubstring("not an array"))
}

// TestExecute_NoArray — the original missing-input check still fires
// when nothing's wired in.
func TestExecute_NoArray(t *testing.T) {
	RegisterTestingT(t)
	result, err := Execute(&core.Flow{}, nil, []*core.Connection{})
	Expect(err).NotTo(HaveOccurred())
	Expect(result["success"]).To(BeFalse())
	Expect(result["error"]).To(ContainSubstring("array is required"))
}

// TestExecute_MalformedJSON — when input is a string but isn't
// valid JSON, surface a clean error rather than mis-counting.
func TestExecute_MalformedJSON(t *testing.T) {
	RegisterTestingT(t)
	result, err := Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "array", Type: core.ConnectionTypeObject, Value: `{not valid json`},
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(result["success"]).To(BeFalse())
	Expect(result["error"]).To(ContainSubstring("not valid JSON"))
}
