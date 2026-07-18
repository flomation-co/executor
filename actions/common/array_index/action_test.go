package array_index

import (
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func run(t *testing.T, value interface{}, idx int) (map[string]interface{}, error) {
	return Execute(&core.Flow{}, nil, []*core.Connection{
		{Name: "array", Type: core.ConnectionTypeObject, Value: value},
		{Name: "index", Type: core.ConnectionTypeInteger, Value: idx},
	})
}

// Regression: AWS (and other) actions return native Go slices like []string, not
// []interface{}. array_index must index into any slice type, not just
// []interface{} — previously []string failed with "expected array, got []string".
func TestIndexNativeStringSlice(t *testing.T) {
	RegisterTestingT(t)

	out, err := run(t, []string{"i-028112c8a056154d4", "i-999"}, 0)
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["item"]).To(Equal("i-028112c8a056154d4"))
}

func TestIndexInterfaceSliceStillWorks(t *testing.T) {
	RegisterTestingT(t)

	out, err := run(t, []interface{}{"a", "b", "c"}, 2)
	Expect(err).To(BeNil())
	Expect(out["item"]).To(Equal("c"))
}

func TestIndexNativeIntSlice(t *testing.T) {
	RegisterTestingT(t)

	out, err := run(t, []int{10, 20, 30}, 1)
	Expect(err).To(BeNil())
	Expect(out["item"]).To(Equal(20))
}

func TestNonArrayStillErrors(t *testing.T) {
	RegisterTestingT(t)

	out, _ := run(t, 42, 0)
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("expected array"))
}
