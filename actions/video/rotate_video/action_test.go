package rotate_video

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestFilterChain(t *testing.T) {
	RegisterTestingT(t)
	Expect(filterChain("90", "none")).To(Equal("transpose=1"))
	Expect(filterChain("180", "none")).To(Equal("transpose=2,transpose=2"))
	Expect(filterChain("270", "none")).To(Equal("transpose=2"))
	Expect(filterChain("0", "horizontal")).To(Equal("hflip"))
	Expect(filterChain("90", "vertical")).To(Equal("transpose=1,vflip"))
	Expect(filterChain("0", "none")).To(Equal(""))
}
