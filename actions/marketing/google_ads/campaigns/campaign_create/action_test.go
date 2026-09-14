package campaign_create

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	core "flomation.app/automate/executor"
	gads "flomation.app/automate/executor/actions/marketing/google_ads"
	. "github.com/onsi/gomega"
)

func ins(pairs ...[2]string) []*core.Connection {
	out := make([]*core.Connection, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, &core.Connection{Name: p[0], Type: core.ConnectionTypeString, Value: p[1]})
	}
	return out
}

// validInputs builds a working input set with the named fields REPLACED.
//
// Replaced, not appended: core.FindConnection returns the first connection with
// a given name, so appending an override leaves the base value in front of it
// and the override silently does nothing. That is exactly what happened when
// these tests were first written — two of them sailed past their validation
// checks and made real calls to googleads.googleapis.com, which is why TestMain
// below now makes an unstubbed call impossible.
func validInputs(extra ...[2]string) []*core.Connection {
	values := map[string]string{
		"credential":       "ya29.token",
		"customer_id":      "123-456-7890",
		"name":             "Brand Search",
		"channel_type":     "SEARCH",
		"daily_budget":     "50.00",
		"bidding_strategy": "TARGET_SPEND",
	}
	order := []string{"credential", "customer_id", "name", "channel_type", "daily_budget", "bidding_strategy"}

	for _, e := range extra {
		if _, exists := values[e[0]]; !exists {
			order = append(order, e[0])
		}
		values[e[0]] = e[1]
	}

	pairs := make([][2]string, 0, len(order))
	for _, name := range order {
		if values[name] == "" {
			continue
		}
		pairs = append(pairs, [2]string{name, values[name]})
	}
	return ins(pairs...)
}

// TestMain points the client at an unroutable address so any test that forgets
// to stand up a stub fails on a refused connection rather than quietly calling
// the live Google Ads API with whatever credential is lying around.
func TestMain(m *testing.M) {
	gads.BaseURL = "http://127.0.0.1:1"
	os.Exit(m.Run())
}

// captureMutate stands in for googleAds:mutate and records the request body.
func captureMutate(t *testing.T, body *map[string]interface{}, response string) func() {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		RegisterTestingT(t)
		Expect(r.URL.Path).To(Equal("/customers/1234567890/googleAds:mutate"),
			"a campaign and its budget must go through the ATOMIC mutate, not two separate calls")
		Expect(json.NewDecoder(r.Body).Decode(body)).To(BeNil())
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, response)
	}))
	original := gads.BaseURL
	gads.BaseURL = server.URL
	return func() {
		gads.BaseURL = original
		server.Close()
	}
}

const twoResults = `{"results":[
  {"campaignBudgetResult":{"resourceName":"customers/1234567890/campaignBudgets/555"}},
  {"campaignResult":{"resourceName":"customers/1234567890/campaigns/777"}}
]}`

// The heart of this action: a campaign references its budget by resource name,
// and the budget does not exist yet. The temporary NEGATIVE id is what lets both
// be created in one transaction, and the campaign must point at exactly the
// placeholder the budget declared — a mismatch is silently a dangling reference.
func TestExecute_WiresTheBudgetTempIdIntoTheCampaign(t *testing.T) {
	RegisterTestingT(t)

	var body map[string]interface{}
	defer captureMutate(t, &body, twoResults)()

	out, err := Execute(nil, nil, validInputs())
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))

	ops, _ := body["mutateOperations"].([]interface{})
	Expect(ops).To(HaveLen(2), "one budget operation and one campaign operation")

	budgetOp, _ := ops[0].(map[string]interface{})["campaignBudgetOperation"].(map[string]interface{})
	budget, _ := budgetOp["create"].(map[string]interface{})
	Expect(budget["resourceName"]).To(Equal("customers/1234567890/campaignBudgets/-1"))

	campaignOp, _ := ops[1].(map[string]interface{})["campaignOperation"].(map[string]interface{})
	campaign, _ := campaignOp["create"].(map[string]interface{})
	Expect(campaign["campaignBudget"]).To(Equal(budget["resourceName"]),
		"the campaign must reference the budget's temporary resource name exactly")

	// Nothing may be committed unless all of it validates: a campaign whose
	// budget failed is a broken account state, not a partial success.
	Expect(body["partialFailure"]).To(Equal(false))
}

// Budgets go to Google in MICROS — 1,000,000 to the pound. Sending minor units
// here (the Meta Ads convention) would be a 10,000x error on live spend.
func TestExecute_SendsTheBudgetInMicros(t *testing.T) {
	RegisterTestingT(t)

	var body map[string]interface{}
	defer captureMutate(t, &body, twoResults)()

	_, err := Execute(nil, nil, validInputs())
	Expect(err).To(BeNil())

	ops, _ := body["mutateOperations"].([]interface{})
	budgetOp, _ := ops[0].(map[string]interface{})["campaignBudgetOperation"].(map[string]interface{})
	budget, _ := budgetOp["create"].(map[string]interface{})

	Expect(budget["amountMicros"]).To(Equal("50000000"), "£50.00 is 50,000,000 micros")
	Expect(budget["explicitlyShared"]).To(Equal(false))
	Expect(budget["name"]).To(Equal("Brand Search budget"))
}

// Against Google's own default of ENABLED. A campaign created by an automation
// — very possibly by an agent — must not start spending the moment it exists.
func TestExecute_DefaultsToPaused(t *testing.T) {
	RegisterTestingT(t)

	var body map[string]interface{}
	defer captureMutate(t, &body, twoResults)()

	_, err := Execute(nil, nil, validInputs())
	Expect(err).To(BeNil())

	ops, _ := body["mutateOperations"].([]interface{})
	campaignOp, _ := ops[1].(map[string]interface{})["campaignOperation"].(map[string]interface{})
	campaign, _ := campaignOp["create"].(map[string]interface{})
	Expect(campaign["status"]).To(Equal("PAUSED"))

	// An explicit choice still wins.
	var enabled map[string]interface{}
	defer captureMutate(t, &enabled, twoResults)()
	_, err = Execute(nil, nil, validInputs([2]string{"status", "ENABLED"}))
	Expect(err).To(BeNil())
	ops, _ = enabled["mutateOperations"].([]interface{})
	campaignOp, _ = ops[1].(map[string]interface{})["campaignOperation"].(map[string]interface{})
	campaign, _ = campaignOp["create"].(map[string]interface{})
	Expect(campaign["status"]).To(Equal("ENABLED"))
}

// The atomic mutate returns results in operation order, so the budget is first.
// A downstream node wants the CAMPAIGN id, not the budget's.
func TestExecute_ReturnsTheCampaignIdNotTheBudgetId(t *testing.T) {
	RegisterTestingT(t)

	var body map[string]interface{}
	defer captureMutate(t, &body, twoResults)()

	out, err := Execute(nil, nil, validInputs())
	Expect(err).To(BeNil())

	Expect(out["id"]).To(Equal("777"))
	Expect(out["resource_name"]).To(Equal("customers/1234567890/campaigns/777"))
	Expect(out["budget_resource_name"]).To(Equal("customers/1234567890/campaignBudgets/555"))
}

// The bidding strategy is a nested object, not the biddingStrategyType enum,
// which is output only. Setting the enum yields a campaign with no strategy.
func TestExecute_SetsBiddingAsANestedObject(t *testing.T) {
	RegisterTestingT(t)

	var body map[string]interface{}
	defer captureMutate(t, &body, twoResults)()

	_, err := Execute(nil, nil, validInputs(
		[2]string{"bidding_strategy", "TARGET_CPA"},
		[2]string{"target_cpa", "12.50"},
	))
	Expect(err).To(BeNil())

	ops, _ := body["mutateOperations"].([]interface{})
	campaignOp, _ := ops[1].(map[string]interface{})["campaignOperation"].(map[string]interface{})
	campaign, _ := campaignOp["create"].(map[string]interface{})

	Expect(campaign["targetCpa"]).To(Equal(map[string]interface{}{"targetCpaMicros": "12500000"}))
	Expect(campaign).ToNot(HaveKey("biddingStrategyType"))
}

// v25 uses start_date_time / end_date_time in "yyyy-MM-dd HH:mm:ss", replacing
// the older bare-date fields. The day is bounded at both ends.
func TestExecute_UsesDateTimeFieldsWithDayBoundaries(t *testing.T) {
	RegisterTestingT(t)

	var body map[string]interface{}
	defer captureMutate(t, &body, twoResults)()

	_, err := Execute(nil, nil, validInputs(
		[2]string{"start_date", "2026-10-01"},
		[2]string{"end_date", "2026-10-31"},
	))
	Expect(err).To(BeNil())

	ops, _ := body["mutateOperations"].([]interface{})
	campaignOp, _ := ops[1].(map[string]interface{})["campaignOperation"].(map[string]interface{})
	campaign, _ := campaignOp["create"].(map[string]interface{})

	Expect(campaign["startDateTime"]).To(Equal("2026-10-01 00:00:00"))
	Expect(campaign["endDateTime"]).To(Equal("2026-10-31 23:59:59"))
	Expect(campaign).ToNot(HaveKey("startDate"))
}

// A dry run must be impossible to mistake for a real creation.
func TestExecute_DryRunIsMarkedAndSentAsValidateOnly(t *testing.T) {
	RegisterTestingT(t)

	var body map[string]interface{}
	defer captureMutate(t, &body, `{"results":[]}`)()

	out, err := Execute(nil, nil, validInputs([2]string{"validate_only", "true"}))
	Expect(err).To(BeNil())

	Expect(body["validateOnly"]).To(Equal(true))
	Expect(out["dry_run"]).To(Equal(true))
	Expect(out["tool_result"]).To(ContainSubstring("NOTHING was applied"))
}

// Campaign types needing asset groups or a linked account would always fail
// from these inputs, so they are refused with the reason rather than attempted.
func TestExecute_RefusesCampaignTypesItCannotBuild(t *testing.T) {
	RegisterTestingT(t)

	out, err := Execute(nil, nil, validInputs([2]string{"channel_type", "PERFORMANCE_MAX"}))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("asset groups"))

	out, err = Execute(nil, nil, validInputs([2]string{"channel_type", "SHOPPING"}))
	Expect(err).To(BeNil())
	Expect(out["error"]).To(ContainSubstring("Merchant Center"))
}

// Validation failures must not reach the network, and must say what to fix.
func TestExecute_ValidatesBeforeCalling(t *testing.T) {
	RegisterTestingT(t)

	// TestMain has already pointed BaseURL at an unroutable address, so any of
	// these reaching the network fails loudly instead of passing by accident.
	for _, c := range []struct {
		name    string
		inputs  []*core.Connection
		wantMsg string
	}{
		{"no budget", validInputs([2]string{"daily_budget", ""}), "daily budget"},
		{"zero budget", validInputs([2]string{"daily_budget", "0"}), "greater than zero"},
		{"no name", validInputs([2]string{"name", ""}), "campaign name"},
		{"bad start date", validInputs([2]string{"start_date", "01/10/2026"}), "YYYY-MM-DD"},
		{"target CPA without an amount", validInputs([2]string{"bidding_strategy", "TARGET_CPA"}), "target CPA"},
	} {
		out, err := Execute(nil, nil, c.inputs)
		Expect(err).To(BeNil(), c.name)
		Expect(out["success"]).To(Equal(false), c.name)
		Expect(out["error"]).To(ContainSubstring(c.wantMsg), c.name)
	}
}

// A Google rejection must arrive as a readable failure rather than "400".
func TestExecute_SurfacesGoogleAdsFailureDetail(t *testing.T) {
	RegisterTestingT(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":{"code":400,"message":"Request contains an invalid argument.","details":[{"errors":[
		  {"errorCode":{"campaignError":"DUPLICATE_CAMPAIGN_NAME"},"message":"A campaign with this name already exists.",
		   "location":{"fieldPathElements":[{"fieldName":"operations","index":1},{"fieldName":"create"},{"fieldName":"name"}]}}
		]}]}}`)
	}))
	defer server.Close()

	original := gads.BaseURL
	gads.BaseURL = server.URL
	defer func() { gads.BaseURL = original }()

	out, err := Execute(nil, nil, validInputs())
	Expect(err).To(BeNil(), "an API rejection is a result, not a node failure")
	Expect(out["success"]).To(Equal(false))

	msg, _ := out["error"].(string)
	Expect(msg).To(ContainSubstring("operation 1"))
	Expect(msg).To(ContainSubstring("create.name"))
	Expect(msg).To(ContainSubstring("DUPLICATE_CAMPAIGN_NAME"))
}
