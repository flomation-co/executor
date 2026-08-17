package campaign_create

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	core "flomation.app/automate/executor"
	meta "flomation.app/automate/executor/actions/marketing/meta_ads"
	. "github.com/onsi/gomega"
)

func in(pairs ...[2]string) []*core.Connection {
	out := make([]*core.Connection, 0, len(pairs))
	for _, p := range pairs {
		t := core.ConnectionTypeString
		switch p[0] {
		case "daily_budget", "lifetime_budget":
			t = core.ConnectionTypeMoney
		case "access_token", "app_secret":
			t = core.ConnectionTypeSecret
		}
		out = append(out, &core.Connection{Name: p[0], Type: t, Value: p[1]})
	}
	return out
}

// stub serves the two calls a budgeted create makes: the account currency
// lookup, then the campaign POST. It records the POST body for inspection.
func stub(t *testing.T, currency string) (*httptest.Server, *url.Values) {
	t.Helper()
	var posted url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": "act_1", "currency": currency})
			return
		}
		_ = r.ParseForm()
		posted = r.PostForm
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": "23851234567890123"})
	}))
	old := meta.BaseURL
	meta.BaseURL = srv.URL
	t.Cleanup(func() { meta.BaseURL = old; srv.Close() })
	return srv, &posted
}

// The money path is the one where a mistake is not a bug report but an
// overspend: Meta takes budgets as minor-unit integers, so £50.00 must go as
// 5000 and never as 50.
func TestExecute_BudgetIsConvertedToMinorUnits(t *testing.T) {
	RegisterTestingT(t)
	_, posted := stub(t, "GBP")

	out, err := Execute(nil, nil, in(
		[2]string{"access_token", "EAAG"},
		[2]string{"account_id", "1234567890"},
		[2]string{"name", "Autumn push"},
		[2]string{"objective", "OUTCOME_LEADS"},
		[2]string{"daily_budget", "50.00"},
	))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(posted.Get("daily_budget")).To(Equal("5000"))
	Expect(out["currency"]).To(Equal("GBP"))
}

// A zero-decimal currency must not be multiplied by 100. ¥5000 is 5000, and
// sending 500000 would be a hundredfold overspend.
func TestExecute_ZeroDecimalCurrencyIsNotScaled(t *testing.T) {
	RegisterTestingT(t)
	_, posted := stub(t, "JPY")

	_, err := Execute(nil, nil, in(
		[2]string{"access_token", "EAAG"},
		[2]string{"account_id", "act_1234567890"},
		[2]string{"name", "Tokyo"},
		[2]string{"objective", "OUTCOME_SALES"},
		[2]string{"daily_budget", "5000"},
	))

	Expect(err).To(BeNil())
	Expect(posted.Get("daily_budget")).To(Equal("5000"))
}

// Creating a campaign that immediately starts spending is not a reasonable
// default for an automated action.
func TestExecute_DefaultsToPaused(t *testing.T) {
	RegisterTestingT(t)
	_, posted := stub(t, "GBP")

	out, err := Execute(nil, nil, in(
		[2]string{"access_token", "EAAG"},
		[2]string{"account_id", "1234567890"},
		[2]string{"name", "Draft"},
		[2]string{"objective", "OUTCOME_TRAFFIC"},
	))

	Expect(err).To(BeNil())
	Expect(posted.Get("status")).To(Equal("PAUSED"))
	Expect(out["tool_result"]).To(ContainSubstring("will not spend"))
	// No budget was set, so the currency lookup must be skipped entirely.
	Expect(out["currency"]).To(Equal(""))
}

// special_ad_categories is required by Meta on every create and must be a JSON
// array; omitting it is rejected outright.
func TestExecute_AlwaysSendsSpecialAdCategories(t *testing.T) {
	RegisterTestingT(t)
	_, posted := stub(t, "GBP")

	_, err := Execute(nil, nil, in(
		[2]string{"access_token", "EAAG"},
		[2]string{"account_id", "1"},
		[2]string{"name", "x"},
		[2]string{"objective", "OUTCOME_AWARENESS"},
	))
	Expect(err).To(BeNil())
	Expect(posted.Get("special_ad_categories")).To(Equal(`["NONE"]`))

	_, posted2 := stub(t, "GBP")
	_, err = Execute(nil, nil, in(
		[2]string{"access_token", "EAAG"},
		[2]string{"account_id", "1"},
		[2]string{"name", "x"},
		[2]string{"objective", "OUTCOME_LEADS"},
		[2]string{"special_ad_categories", "EMPLOYMENT"},
	))
	Expect(err).To(BeNil())
	Expect(posted2.Get("special_ad_categories")).To(Equal(`["EMPLOYMENT"]`))
}

func TestExecute_RequiresTheEssentials(t *testing.T) {
	RegisterTestingT(t)
	stub(t, "GBP")

	for _, missing := range []string{"account_id", "name", "objective"} {
		fields := map[string]string{
			"access_token": "EAAG", "account_id": "1", "name": "x", "objective": "OUTCOME_LEADS",
		}
		delete(fields, missing)
		pairs := make([][2]string, 0, len(fields))
		for k, v := range fields {
			pairs = append(pairs, [2]string{k, v})
		}
		out, err := Execute(nil, nil, in(pairs...))
		Expect(err).To(BeNil())
		Expect(out["success"]).To(Equal(false), "missing %s should fail", missing)
	}
}

// A Graph error must surface as a graceful failure the flow can branch on, not
// a node error — and it must carry Meta's own explanation.
func TestExecute_SurfacesGraphErrorsGracefully(t *testing.T) {
	RegisterTestingT(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": map[string]interface{}{
			"code": 100, "message": "Invalid parameter",
			"error_user_msg": "Your daily budget is below the minimum for GBP",
		}})
	}))
	old := meta.BaseURL
	meta.BaseURL = srv.URL
	defer func() { meta.BaseURL = old; srv.Close() }()

	out, err := Execute(nil, nil, in(
		[2]string{"access_token", "EAAG"},
		[2]string{"account_id", "1"},
		[2]string{"name", "x"},
		[2]string{"objective", "OUTCOME_LEADS"},
	))

	Expect(err).To(BeNil(), "a Graph rejection is a flow-level result, not a node error")
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("below the minimum for GBP"))
}

// The dangerous failure here is silent and hundredfold: an AI that knows Meta
// takes minor units may pre-convert £10 to 1000, which is then multiplied again
// into 100000 and books £1,000 a day. Nothing errors. Echoing the exact integer
// sent alongside the major-unit input makes it visible immediately, rather than
// after the money has gone.
func TestExecute_ReportsExactMinorUnitsSent(t *testing.T) {
	RegisterTestingT(t)
	_, posted := stub(t, "GBP")

	out, err := Execute(nil, nil, in(
		[2]string{"access_token", "EAAG"},
		[2]string{"account_id", "1234567890"},
		[2]string{"name", "Test"},
		[2]string{"objective", "OUTCOME_LEADS"},
		[2]string{"daily_budget", "10.00"},
	))

	Expect(err).To(BeNil())
	Expect(posted.Get("daily_budget")).To(Equal("1000"))

	summary, _ := out["tool_result"].(string)
	Expect(summary).To(ContainSubstring("10.00 GBP"), "the major-unit input must be echoed")
	Expect(summary).To(ContainSubstring("sent to Meta as 1000"), "the exact integer must be echoed")
	// A pre-converted value would show as 100000 here, which is the whole point.
	Expect(summary).ToNot(ContainSubstring("100000"))
}

// The pre-converted case: an agent passing 1000 for "£10" produces £1,000/day.
// The action cannot know it was a mistake, but it must make it visible.
func TestExecute_PreConvertedBudgetIsVisibleInTheResult(t *testing.T) {
	RegisterTestingT(t)
	_, posted := stub(t, "GBP")

	out, err := Execute(nil, nil, in(
		[2]string{"access_token", "EAAG"},
		[2]string{"account_id", "1"},
		[2]string{"name", "Test"},
		[2]string{"objective", "OUTCOME_LEADS"},
		[2]string{"daily_budget", "1000"}, // meant £10, passed pence
	))

	Expect(err).To(BeNil())
	Expect(posted.Get("daily_budget")).To(Equal("100000"))

	summary, _ := out["tool_result"].(string)
	Expect(summary).To(ContainSubstring("sent to Meta as 100000"),
		"a hundredfold error must be legible in the result, not discovered later")
}
