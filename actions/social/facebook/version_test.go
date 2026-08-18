package facebook_common

import (
	"strconv"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

// The Graph version must stay on a supported release.
//
// v19.0 expired on 21 May 2026 and stayed pinned here for months without anyone
// noticing, because Meta does NOT hard-fail an expired version — it silently
// routes the call elsewhere. Nothing errored, nothing logged, and the effective
// request surface drifted away from what the code was written against. A clean
// failure would have been better; since there isn't one, this test is the
// signal instead.
//
// Meta expires a version roughly two years after release, so this is a periodic
// chore, not a one-off. Bump the floor when the pin moves.
func TestGraphAPIVersionIsSupported(t *testing.T) {
	RegisterTestingT(t)

	const prefix = "https://graph.facebook.com/v"
	Expect(GraphAPIBase).To(HavePrefix(prefix), "the Graph base URL must carry an explicit version")

	version := strings.TrimPrefix(GraphAPIBase, prefix)
	major, _, found := strings.Cut(version, ".")
	Expect(found).To(BeTrue(), "version %q should look like <major>.<minor>", version)

	n, err := strconv.Atoi(major)
	Expect(err).To(BeNil(), "version %q should start with a major number", version)

	// v22.0 (released Jan 2025) expires May 2027 — anything older is either
	// already expired or close enough that it should not be pinned in new code.
	Expect(n).To(BeNumerically(">=", 22),
		"Graph v%d is expired or near expiry. Meta silently reroutes expired versions rather than failing, "+
			"so this will not show up as an error in production — check "+
			"https://developers.facebook.com/docs/graph-api/changelog/versions and bump both the pin and this floor.", n)
}
