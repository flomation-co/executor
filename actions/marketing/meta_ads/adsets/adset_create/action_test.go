package adset_create

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	meta "flomation.app/automate/executor/actions/marketing/meta_ads"
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

// Meta reports the campaign-vs-ad-set budget conflict only AFTER the attempt,
// and never says which side already has a budget. Worse, a CBO campaign
// commonly also carries a cap-requiring bid strategy, so the two failures land
// together — and fixing either alone still fails, which makes each correct fix
// look like a wrong one. Both are named in one message.
func TestCampaignBudgetConflict(t *testing.T) {
	RegisterTestingT(t)

	campaign := func(fields map[string]interface{}) *httptest.Server {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(fields)
		}))
		old := meta.BaseURL
		meta.BaseURL = srv.URL
		t.Cleanup(func() { meta.BaseURL = old; srv.Close() })
		return srv
	}

	// CBO with a cap-requiring bid strategy: both constraints named at once.
	campaign(map[string]interface{}{"id": "1", "daily_budget": "1000", "bid_strategy": "LOWEST_COST_WITH_BID_CAP"})
	msg := meta.CampaignBudgetConflict(nil, meta.NewClient("t", ""), "1", true)
	Expect(msg).To(ContainSubstring("already has a daily budget"))
	Expect(msg).To(ContainSubstring("1000"))
	Expect(msg).To(ContainSubstring("one or the other, not both"))
	Expect(msg).To(ContainSubstring("LOWEST_COST_WITH_BID_CAP"))
	Expect(msg).To(ContainSubstring("needs a Bid Cap"))

	// Lifetime budget is the same conflict, described correctly.
	campaign(map[string]interface{}{"id": "1", "lifetime_budget": "50000"})
	Expect(meta.CampaignBudgetConflict(nil, meta.NewClient("t", ""), "1", true)).
		To(ContainSubstring("already has a lifetime budget"))

	// No campaign budget → no conflict.
	campaign(map[string]interface{}{"id": "1", "bid_strategy": "LOWEST_COST_WITHOUT_CAP"})
	Expect(meta.CampaignBudgetConflict(nil, meta.NewClient("t", ""), "1", true)).To(Equal(""))

	// The ad set sets no budget → nothing to conflict with, and no call made.
	campaign(map[string]interface{}{"id": "1", "daily_budget": "1000"})
	Expect(meta.CampaignBudgetConflict(nil, meta.NewClient("t", ""), "1", false)).To(Equal(""))
}

// A campaign that cannot be read must not block the create — the pre-check is a
// courtesy, and Meta's own answer is authoritative.
func TestCampaignBudgetConflict_FailedLookupDoesNotBlock(t *testing.T) {
	RegisterTestingT(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"code":200,"message":"No permission"}}`))
	}))
	old := meta.BaseURL
	meta.BaseURL = srv.URL
	defer func() { meta.BaseURL = old; srv.Close() }()

	Expect(meta.CampaignBudgetConflict(nil, meta.NewClient("t", ""), "1", true)).To(Equal(""))
}
