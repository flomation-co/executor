package adset_create

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

// The documented rule: these two strategies require a cap by definition.
func TestBidRequirementError_CapStrategiesNeedACap(t *testing.T) {
	RegisterTestingT(t)

	for _, strategy := range []string{"LOWEST_COST_WITH_BID_CAP", "COST_CAP"} {
		msg := bidRequirementError("LINK_CLICKS", "LINK_CLICKS", strategy, "")
		Expect(msg).To(ContainSubstring(strategy))
		Expect(msg).To(ContainSubstring("requires a Bid Cap"))
		Expect(msg).To(ContainSubstring("LOWEST_COST_WITHOUT_CAP"), "the message must name the way out")

		// Supplying the cap satisfies it.
		Expect(bidRequirementError("LINK_CLICKS", "LINK_CLICKS", strategy, "1.00")).To(Equal(""))
	}
}

// The undocumented pairing that cost two round trips and a wrong diagnosis:
// Meta objects to the BID while the reader is looking at the BUDGET.
func TestBidRequirementError_LinkClicksOnImpressions(t *testing.T) {
	RegisterTestingT(t)

	msg := bidRequirementError("LINK_CLICKS", "IMPRESSIONS", "", "")
	Expect(msg).To(ContainSubstring("Bid Cap"))

	// Must state that billing event is a real cost decision, not a workaround —
	// an agent described switching it as functionally cost-neutral, which is
	// false and would misinform a production budget decision.
	Expect(msg).To(ContainSubstring("CHARGED"))
	Expect(msg).To(ContainSubstring("per 1,000 impressions"))
	Expect(msg).To(ContainSubstring("per click"))

	// Must kill the budget red herring explicitly.
	Expect(msg).To(ContainSubstring("does NOT lift this requirement"))

	// And must point at the parent campaign, since a cap-requiring bid strategy
	// there forces a cap on every ad set regardless of these settings.
	Expect(msg).To(ContainSubstring("PARENT CAMPAIGN"))
	Expect(msg).To(ContainSubstring("Campaigns: List"))
}

// A cap supplied, or a combination Meta accepts, must pass silently — a
// false refusal here blocks a legitimate ad set.
func TestBidRequirementError_AllowsValidCombinations(t *testing.T) {
	RegisterTestingT(t)

	Expect(bidRequirementError("LINK_CLICKS", "IMPRESSIONS", "", "0.50")).To(Equal(""))
	Expect(bidRequirementError("LINK_CLICKS", "LINK_CLICKS", "", "")).To(Equal(""))
	Expect(bidRequirementError("REACH", "IMPRESSIONS", "", "")).To(Equal(""))
	Expect(bidRequirementError("LANDING_PAGE_VIEWS", "IMPRESSIONS", "", "")).To(Equal(""))
	Expect(bidRequirementError("", "", "", "")).To(Equal(""))
	Expect(bidRequirementError("LINK_CLICKS", "IMPRESSIONS", "LOWEST_COST_WITHOUT_CAP", "0.50")).To(Equal(""))

	// Whitespace-only is not a cap.
	Expect(bidRequirementError("LINK_CLICKS", "IMPRESSIONS", "", "   ")).ToNot(Equal(""))
}

// The message is what a human or an agent acts on, so it has to name the fix
// rather than restate the failure.
func TestBidRequirementError_MessagesAreActionable(t *testing.T) {
	RegisterTestingT(t)

	for _, msg := range []string{
		bidRequirementError("LINK_CLICKS", "IMPRESSIONS", "", ""),
		bidRequirementError("LINK_CLICKS", "LINK_CLICKS", "COST_CAP", ""),
	} {
		Expect(msg).ToNot(BeEmpty())
		Expect(len(strings.Fields(msg))).To(BeNumerically(">", 12), "a one-line restatement is not guidance")
		Expect(msg).To(MatchRegexp("Set a Bid Cap|Set one"), "must say what to do")
	}
}
