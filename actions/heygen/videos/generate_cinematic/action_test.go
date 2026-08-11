package generate_cinematic

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestSplitIDs(t *testing.T) {
	RegisterTestingT(t)
	Expect(splitIDs("look_1, look_2 ,look_3")).To(Equal([]string{"look_1", "look_2", "look_3"}))
	Expect(splitIDs("  solo  ")).To(Equal([]string{"solo"}))
	Expect(splitIDs("a,,b,")).To(Equal([]string{"a", "b"}))
	Expect(splitIDs("")).To(BeEmpty())
}
