package download

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

func TestTruncateForTool(t *testing.T) {
	RegisterTestingT(t)

	// Short text is returned verbatim (no truncation note).
	short := "Question 1: describe your approach."
	Expect(truncateForTool(short)).To(Equal(short))

	// Oversized text is capped, keeps the head, and flags the truncation.
	big := strings.Repeat("x", 60000)
	out := truncateForTool(big)
	Expect(len([]rune(out))).To(BeNumerically("<", 60000))
	Expect(out).To(HavePrefix(strings.Repeat("x", 100)))
	Expect(out).To(ContainSubstring("truncated 50000 of 60000 characters"))
	Expect(out).To(ContainSubstring("content"))
}
