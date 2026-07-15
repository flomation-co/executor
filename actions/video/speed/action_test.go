package speed

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestBuildAtempo_ChainsWithinRange(t *testing.T) {
	RegisterTestingT(t)
	Expect(buildAtempo(2.0)).To(Equal("atempo=2"))
	Expect(buildAtempo(0.5)).To(Equal("atempo=0.5"))
	Expect(buildAtempo(1.5)).To(Equal("atempo=1.5"))
	Expect(buildAtempo(4.0)).To(Equal("atempo=2.0,atempo=2"))    // 2 * 2
	Expect(buildAtempo(0.25)).To(Equal("atempo=0.5,atempo=0.5")) // 0.5 * 0.5
}
