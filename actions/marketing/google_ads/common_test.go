package google_ads_common

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func ins(pairs ...[2]string) []*core.Connection {
	out := make([]*core.Connection, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, &core.Connection{Name: p[0], Type: core.ConnectionTypeString, Value: p[1]})
	}
	return out
}

// --- money ---

// Micros, not minor units. The whole reason this helper exists rather than
// reusing stripe_common.MoneyToMinorUnits is that the two differ by a factor of
// 10,000 for a two-decimal currency, and getting it wrong spends real money at
// the wrong scale.
func TestMoneyToMicros(t *testing.T) {
	RegisterTestingT(t)

	cases := []struct {
		in   string
		want int64
	}{
		{"50", 50000000},
		{"50.00", 50000000},
		{"5.43", 5430000},
		{"0.01", 10000},
		{"0.000001", 1},
		{"1234.56", 1234560000},
	}
	for _, c := range cases {
		got, err := MoneyToMicros("amount", ins([2]string{"amount", c.in}))
		Expect(err).To(BeNil(), c.in)
		Expect(got).ToNot(BeNil(), c.in)
		Expect(*got).To(Equal(c.want), "%s should be %d micros", c.in, c.want)
	}
}

// 0.1 is not representable in binary floating point, so a float-based
// conversion drifts. A budget that is a micro out every time is the sort of
// defect nobody finds until it is reconciled against an invoice.
func TestMoneyToMicros_IsExactForRepeatingBinaryFractions(t *testing.T) {
	RegisterTestingT(t)

	for _, c := range []struct {
		in   string
		want int64
	}{{"0.1", 100000}, {"0.7", 700000}, {"29.99", 29990000}, {"0.29", 290000}} {
		got, err := MoneyToMicros("amount", ins([2]string{"amount", c.in}))
		Expect(err).To(BeNil())
		Expect(*got).To(Equal(c.want), "%s must convert exactly", c.in)
	}
}

// Blank must stay unset rather than becoming zero: zero is a validation error at
// Google, but "I did not set this" should mean the field is omitted.
func TestMoneyToMicros_BlankIsNilNotZero(t *testing.T) {
	RegisterTestingT(t)

	got, err := MoneyToMicros("amount", ins([2]string{"amount", ""}))
	Expect(err).To(BeNil())
	Expect(got).To(BeNil())

	got, err = MoneyToMicros("amount", ins())
	Expect(err).To(BeNil())
	Expect(got).To(BeNil())
}

// A ${...} substitution or an agent may deliver a symbol or separators.
func TestMoneyToMicros_TolerantOfFormatting(t *testing.T) {
	RegisterTestingT(t)

	got, err := MoneyToMicros("amount", ins([2]string{"amount", "£1,234.56"}))
	Expect(err).To(BeNil())
	Expect(*got).To(Equal(int64(1234560000)))
}

func TestMoneyToMicros_RejectsNonsenseAndNegatives(t *testing.T) {
	RegisterTestingT(t)

	_, err := MoneyToMicros("amount", ins([2]string{"amount", "-5.00"}))
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("negative"))

	_, err = MoneyToMicros("amount", ins([2]string{"amount", "fifty pounds"}))
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("major units"))
}

// The read side matters as much as the write side: an unconverted cost has an
// agent reporting "spend was 5,430,000" when it was 5.43.
func TestMicrosToMajor(t *testing.T) {
	RegisterTestingT(t)

	Expect(MicrosToMajor(5430000)).To(Equal("5.43"))
	Expect(MicrosToMajor(50000000)).To(Equal("50"))
	Expect(MicrosToMajor(0)).To(Equal("0"))
	Expect(MicrosToMajor(1)).To(Equal("0.000001"))
	Expect(MicrosToMajor(100000)).To(Equal("0.1"))
	Expect(MicrosToMajor(-5430000)).To(Equal("-5.43"))
}

func TestMoneyRoundTrip(t *testing.T) {
	RegisterTestingT(t)

	for _, amount := range []string{"50", "5.43", "0.1", "1234.56", "0.01"} {
		micros, err := MoneyToMicros("amount", ins([2]string{"amount", amount}))
		Expect(err).To(BeNil())
		// "50" renders back as "50", so compare against the canonical form by
		// converting twice rather than against the input string.
		again, err := MoneyToMicros("amount", ins([2]string{"amount", MicrosToMajor(*micros)}))
		Expect(err).To(BeNil())
		Expect(*again).To(Equal(*micros), "round trip must not drift for %s", amount)
	}
}

// --- customer ids ---

// Google Ads shows 123-456-7890 in its own UI and every header wants
// 1234567890. Pasting from the UI is the common case.
func TestNormaliseCustomerID(t *testing.T) {
	RegisterTestingT(t)

	Expect(NormaliseCustomerID("123-456-7890")).To(Equal("1234567890"))
	Expect(NormaliseCustomerID("1234567890")).To(Equal("1234567890"))
	Expect(NormaliseCustomerID(" 123 456 7890 ")).To(Equal("1234567890"))
	Expect(NormaliseCustomerID("")).To(Equal(""))
	Expect(NormaliseCustomerID("not-a-number")).To(Equal(""))
}

// --- row shaping ---

// Two conversions matter here: GAQL is written in snake_case but the response
// comes back in camelCase, and *_micros gains a major-unit sibling.
func TestFlattenRow(t *testing.T) {
	RegisterTestingT(t)

	row := map[string]interface{}{
		"campaign": map[string]interface{}{
			"resourceName": "customers/1/campaigns/2",
			"id":           "2",
			"name":         "Brand",
		},
		"metrics": map[string]interface{}{
			"clicks":     "5",
			"costMicros": "5430000",
		},
		"campaignBudget": map[string]interface{}{"amountMicros": "50000000"},
	}

	flat := FlattenRow(row)

	// Selected as campaign.name, returned as campaign.name.
	Expect(flat["campaign.name"]).To(Equal("Brand"))
	Expect(flat["campaign.resource_name"]).To(Equal("customers/1/campaigns/2"))
	Expect(flat["metrics.clicks"]).To(Equal("5"))
	Expect(flat["campaign_budget.amount_micros"]).To(Equal("50000000"))

	// Raw micros preserved, major-unit sibling added.
	Expect(flat["metrics.cost_micros"]).To(Equal("5430000"))
	Expect(flat["metrics.cost"]).To(Equal("5.43"))
	Expect(flat["campaign_budget.amount"]).To(Equal("50"))
}

func TestSnakeCase(t *testing.T) {
	RegisterTestingT(t)

	Expect(SnakeCase("costMicros")).To(Equal("cost_micros"))
	Expect(SnakeCase("name")).To(Equal("name"))
	Expect(SnakeCase("advertisingChannelSubType")).To(Equal("advertising_channel_sub_type"))
	// Digits belong to the preceding word: path1 must not become path_1.
	Expect(SnakeCase("path1")).To(Equal("path1"))
}

// --- errors ---

// A failed mutate is an HTTP 400 whose useful content is buried in
// error.details[]. Surfacing "400 Bad Request" turns a self-service fix into a
// support ticket.
func TestDescribeFailure_NamesOperationFieldAndCode(t *testing.T) {
	RegisterTestingT(t)

	body := []byte(`{
	  "error": {
	    "code": 400,
	    "message": "Request contains an invalid argument.",
	    "status": "INVALID_ARGUMENT",
	    "details": [{
	      "@type": "type.googleapis.com/google.ads.googleads.v25.errors.GoogleAdsFailure",
	      "errors": [{
	        "errorCode": {"campaignError": "DUPLICATE_CAMPAIGN_NAME"},
	        "message": "A campaign with this name already exists.",
	        "location": {"fieldPathElements": [
	          {"fieldName": "operations", "index": 2},
	          {"fieldName": "create"},
	          {"fieldName": "name"}
	        ]}
	      }]
	    }]
	  }
	}`)

	var decoded map[string]interface{}
	Expect(json.Unmarshal(body, &decoded)).To(BeNil())

	msg := DescribeFailure(400, decoded, body)

	Expect(msg).To(ContainSubstring("operation 2"), "the caller needs to know WHICH operation failed")
	Expect(msg).To(ContainSubstring("create.name"), "and which field")
	Expect(msg).To(ContainSubstring("A campaign with this name already exists."))
	Expect(msg).To(ContainSubstring("DUPLICATE_CAMPAIGN_NAME"), "the enum is what a user can search for")
}

func TestDescribeFailure_ReportsEveryFailedOperation(t *testing.T) {
	RegisterTestingT(t)

	body := []byte(`{"error":{"code":400,"message":"Invalid","details":[{"errors":[
	  {"errorCode":{"criterionError":"KEYWORD_HAS_INVALID_CHARS"},"message":"Bad chars.",
	   "location":{"fieldPathElements":[{"fieldName":"operations","index":0}]}},
	  {"errorCode":{"criterionError":"KEYWORD_TEXT_TOO_LONG"},"message":"Too long.",
	   "location":{"fieldPathElements":[{"fieldName":"operations","index":4}]}}
	]}]}}`)

	var decoded map[string]interface{}
	Expect(json.Unmarshal(body, &decoded)).To(BeNil())

	msg := DescribeFailure(400, decoded, body)
	Expect(msg).To(ContainSubstring("operation 0"))
	Expect(msg).To(ContainSubstring("operation 4"))
	Expect(msg).To(ContainSubstring("KEYWORD_TEXT_TOO_LONG"))
}

// 401 means the managed credential failed to refresh, so the message must point
// at reconnecting rather than at the request.
func TestDescribeFailure_AuthAndQuotaAreActionable(t *testing.T) {
	RegisterTestingT(t)

	unauth := map[string]interface{}{"error": map[string]interface{}{"message": "Request had invalid authentication credentials."}}
	Expect(DescribeFailure(401, unauth, nil)).To(ContainSubstring("reconnect"))

	forbidden := map[string]interface{}{"error": map[string]interface{}{"message": "The caller does not have permission"}}
	Expect(DescribeFailure(403, forbidden, nil)).To(ContainSubstring("Manager Account ID"))

	quota := map[string]interface{}{"error": map[string]interface{}{"message": "Resource exhausted"}}
	Expect(DescribeFailure(429, quota, nil)).To(ContainSubstring("quota"))
}

func TestDescribeFailure_FallsBackToTheRawBody(t *testing.T) {
	RegisterTestingT(t)

	msg := DescribeFailure(502, nil, []byte("<html>bad gateway</html>"))
	Expect(msg).To(ContainSubstring("502"))
	Expect(msg).To(ContainSubstring("bad gateway"))
}

// --- GAQL ---

// GAQL values cannot be bound as parameters, so escaping here is the only thing
// between a campaign named with an apostrophe and a malformed query — and
// between a hostile value and a rewritten WHERE clause.
func TestQuoteGAQL(t *testing.T) {
	RegisterTestingT(t)

	Expect(QuoteGAQL("Brand")).To(Equal("'Brand'"))
	Expect(QuoteGAQL("Bob's Shoes")).To(Equal(`'Bob\'s Shoes'`))
	Expect(QuoteGAQL(`back\slash`)).To(Equal(`'back\\slash'`))
	Expect(QuoteGAQL("line\nbreak")).To(Equal("'line break'"))

	// An injection attempt must stay inside the literal.
	injected := QuoteGAQL("x' OR campaign.id != '0")
	Expect(injected).To(Equal(`'x\' OR campaign.id != \'0'`))
}

func TestDateRangeClause(t *testing.T) {
	RegisterTestingT(t)

	// Named ranges are unquoted keywords after DURING.
	clause, err := DateRangeClause("LAST_30_DAYS", "", "")
	Expect(err).To(BeNil())
	Expect(clause).To(Equal("segments.date DURING LAST_30_DAYS"))

	// Explicit dates are quoted and use BETWEEN. Getting these the wrong way
	// round is a syntax error rather than a wrong answer.
	clause, err = DateRangeClause("CUSTOM", "2026-01-01", "2026-01-31")
	Expect(err).To(BeNil())
	Expect(clause).To(Equal("segments.date BETWEEN '2026-01-01' AND '2026-01-31'"))

	_, err = DateRangeClause("LAST_FORTNIGHT", "", "")
	Expect(err).ToNot(BeNil())

	_, err = DateRangeClause("CUSTOM", "01/01/2026", "2026-01-31")
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("YYYY-MM-DD"))

	_, err = DateRangeClause("CUSTOM", "2026-01-01", "")
	Expect(err).ToNot(BeNil())
}

// v25 replaced start_date/end_date with start_date_time/end_date_time in
// "yyyy-MM-dd HH:mm:ss" form. Code written against the older shape fails with
// an unhelpful field error rather than a date error.
func TestCampaignDateTime(t *testing.T) {
	RegisterTestingT(t)

	start, err := CampaignDateTime("2026-09-12", false)
	Expect(err).To(BeNil())
	Expect(start).To(Equal("2026-09-12 00:00:00"))

	end, err := CampaignDateTime("2026-09-12", true)
	Expect(err).To(BeNil())
	Expect(end).To(Equal("2026-09-12 23:59:59"))

	// Blank stays blank rather than becoming "today".
	blank, err := CampaignDateTime("", false)
	Expect(err).To(BeNil())
	Expect(blank).To(Equal(""))

	// A complete timestamp is honoured rather than overruled.
	exact, err := CampaignDateTime("2026-09-12 13:30:00", true)
	Expect(err).To(BeNil())
	Expect(exact).To(Equal("2026-09-12 13:30:00"))

	_, err = CampaignDateTime("12/09/2026", false)
	Expect(err).ToNot(BeNil())
}

func TestBuildQuery(t *testing.T) {
	RegisterTestingT(t)

	limit := int64(50)
	q := BuildQuery([]string{"campaign.id, campaign.name", "metrics.clicks"}, "campaign",
		[]string{"campaign.status = 'ENABLED'", "", "segments.date DURING LAST_7_DAYS"}, "campaign.name", &limit)

	Expect(q).To(Equal("SELECT campaign.id, campaign.name, metrics.clicks FROM campaign " +
		"WHERE campaign.status = 'ENABLED' AND segments.date DURING LAST_7_DAYS ORDER BY campaign.name LIMIT 50"))

	// No conditions, no order, no limit.
	Expect(BuildQuery([]string{"customer.id"}, "customer", nil, "", nil)).To(Equal("SELECT customer.id FROM customer"))
}

// --- keywords ---

// Google Ads writes phrase match as "term" and exact as [term], and that is
// what people paste. Leaving the punctuation in creates a keyword whose literal
// text contains brackets, which matches nothing and looks right in the UI.
func TestKeywordTerms_StripsMatchTypePunctuation(t *testing.T) {
	RegisterTestingT(t)

	terms, err := KeywordTerms("[running shoes]\n\"trail shoes\"\nwalking boots")
	Expect(err).To(BeNil())
	Expect(terms).To(Equal([]string{"running shoes", "trail shoes", "walking boots"}))
}

func TestKeywordTerms_SkipsBlanksAndCaseDuplicates(t *testing.T) {
	RegisterTestingT(t)

	terms, err := KeywordTerms("shoes\n\n  \nSHOES\nboots\n")
	Expect(err).To(BeNil())
	// Google treats keywords case-insensitively, so the second is a duplicate
	// that would be rejected and cost an operation.
	Expect(terms).To(Equal([]string{"shoes", "boots"}))
}

func TestKeywordTerms_EnforcesGoogleLimits(t *testing.T) {
	RegisterTestingT(t)

	long := ""
	for i := 0; i < 90; i++ {
		long += "a"
	}
	_, err := KeywordTerms(long)
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("80 character"))

	_, err = KeywordTerms("one two three four five six seven eight nine ten eleven")
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("10 word"))

	_, err = KeywordTerms("   \n  ")
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("one per line"))
}

// --- bidding ---

// A campaign carries its strategy as a nested object, not as the
// biddingStrategyType enum (which is output only). Setting the enum instead
// produces a campaign with no strategy at all.
func TestApplyBiddingStrategy(t *testing.T) {
	RegisterTestingT(t)

	campaign := map[string]interface{}{}
	mask, err := ApplyBiddingStrategy("MANUAL_CPC", ins(), campaign)
	Expect(err).To(BeNil())
	Expect(campaign).To(HaveKey("manualCpc"))
	Expect(campaign).ToNot(HaveKey("biddingStrategyType"))
	Expect(mask).To(Equal([]string{"manual_cpc"}))

	campaign = map[string]interface{}{}
	_, err = ApplyBiddingStrategy("TARGET_CPA", ins([2]string{"target_cpa", "12.50"}), campaign)
	Expect(err).To(BeNil())
	Expect(campaign["targetCpa"]).To(Equal(map[string]interface{}{"targetCpaMicros": "12500000"}))

	campaign = map[string]interface{}{}
	_, err = ApplyBiddingStrategy("TARGET_ROAS", ins([2]string{"target_roas", "4"}), campaign)
	Expect(err).To(BeNil())
	Expect(campaign["targetRoas"]).To(Equal(map[string]interface{}{"targetRoas": float64(4)}))

	// Google's UI calls TargetSpend "Maximise clicks"; both names must work.
	campaign = map[string]interface{}{}
	_, err = ApplyBiddingStrategy("MAXIMIZE_CLICKS", ins(), campaign)
	Expect(err).To(BeNil())
	Expect(campaign).To(HaveKey("targetSpend"))
}

func TestApplyBiddingStrategy_RequiresItsTargets(t *testing.T) {
	RegisterTestingT(t)

	_, err := ApplyBiddingStrategy("TARGET_CPA", ins(), map[string]interface{}{})
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("target CPA"))

	_, err = ApplyBiddingStrategy("TARGET_ROAS", ins(), map[string]interface{}{})
	Expect(err).ToNot(BeNil())

	_, err = ApplyBiddingStrategy("SOMETHING_ELSE", ins(), map[string]interface{}{})
	Expect(err).ToNot(BeNil())

	// Blank means "leave it alone", not an error.
	campaign := map[string]interface{}{}
	mask, err := ApplyBiddingStrategy("", ins(), campaign)
	Expect(err).To(BeNil())
	Expect(mask).To(BeEmpty())
	Expect(campaign).To(BeEmpty())
}

// --- inputs ---

// An unresolved ${...} is not a value. Passing it through to GAQL produces a
// syntax error naming the query rather than the missing variable.
func TestOptionalString_DropsUnresolvedReferences(t *testing.T) {
	RegisterTestingT(t)

	Expect(OptionalString("x", ins([2]string{"x", "${flow.missing}"}))).To(Equal(""))
	Expect(OptionalString("x", ins([2]string{"x", "  value  "}))).To(Equal("value"))
}

func TestOptionalList_AcceptsBothStorageForms(t *testing.T) {
	RegisterTestingT(t)

	Expect(OptionalList("m", ins([2]string{"m", `["metrics.clicks","metrics.impressions"]`}))).
		To(Equal([]string{"metrics.clicks", "metrics.impressions"}))
	Expect(OptionalList("m", ins([2]string{"m", "metrics.clicks, metrics.impressions"}))).
		To(Equal([]string{"metrics.clicks", "metrics.impressions"}))
	Expect(OptionalList("m", ins())).To(BeNil())
}

func TestGetAuth(t *testing.T) {
	RegisterTestingT(t)

	token, customer, login, err := GetAuth(ins(
		[2]string{"credential", "ya29.token"},
		[2]string{"customer_id", "123-456-7890"},
		[2]string{"login_customer_id", "098-765-4321"},
	))
	Expect(err).To(BeNil())
	Expect(token).To(Equal("ya29.token"))
	Expect(customer).To(Equal("1234567890"))
	Expect(login).To(Equal("0987654321"))

	// An unresolved credential must fail with advice, not be sent as a literal
	// bearer token for an opaque 401.
	_, _, _, err = GetAuth(ins([2]string{"credential", "${credentials.Missing}"}, [2]string{"customer_id", "1234567890"}))
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("connect"))

	// An unresolved OPTIONAL manager id means "no manager", not a literal header.
	_, _, login, err = GetAuth(ins(
		[2]string{"credential", "t"},
		[2]string{"customer_id", "1234567890"},
		[2]string{"login_customer_id", "${credentials.X.login_customer_id}"},
	))
	Expect(err).To(BeNil())
	Expect(login).To(Equal(""))

	_, _, _, err = GetAuth(ins([2]string{"credential", "t"}, [2]string{"customer_id", "not-a-number"}))
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("ten-digit"))
}

// --- client ---

func TestSearch_FollowsPagination(t *testing.T) {
	RegisterTestingT(t)

	var bodies []map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		Expect(json.NewDecoder(r.Body).Decode(&body)).To(BeNil())
		bodies = append(bodies, body)

		Expect(r.URL.Path).To(Equal("/customers/1234567890/googleAds:search"))
		Expect(r.Header.Get("Authorization")).To(Equal("Bearer tok"))

		w.Header().Set("Content-Type", "application/json")
		if len(bodies) == 1 {
			fmt.Fprint(w, `{"results":[{"campaign":{"id":"1"}}],"nextPageToken":"page2"}`)
			return
		}
		fmt.Fprint(w, `{"results":[{"campaign":{"id":"2"}}]}`)
	}))
	defer server.Close()

	original := BaseURL
	BaseURL = server.URL
	defer func() { BaseURL = original }()

	rows, err := NewClient("tok", "").Search(nil, "123-456-7890", "SELECT campaign.id FROM campaign")
	Expect(err).To(BeNil())
	Expect(rows).To(HaveLen(2))

	// The second request must carry the token from the first.
	Expect(bodies).To(HaveLen(2))
	Expect(bodies[0]).ToNot(HaveKey("pageToken"))
	Expect(bodies[1]["pageToken"]).To(Equal("page2"))

	// pageSize must never be sent: the API returns PAGE_SIZE_NOT_SUPPORTED.
	Expect(bodies[0]).ToNot(HaveKey("pageSize"))
}

// Developer tokens were sunset on 9 September 2026 and Google promises to start
// rejecting them in a future major version, so the header must not be sent.
func TestClient_SendsLoginCustomerIdButNoDeveloperToken(t *testing.T) {
	RegisterTestingT(t)

	var seen http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Clone()
		fmt.Fprint(w, `{"results":[]}`)
	}))
	defer server.Close()

	original := BaseURL
	BaseURL = server.URL
	defer func() { BaseURL = original }()

	_, err := NewClient("tok", "098-765-4321").Search(nil, "1234567890", "SELECT customer.id FROM customer")
	Expect(err).To(BeNil())

	Expect(seen.Get("login-customer-id")).To(Equal("0987654321"), "dashes must be stripped from the header")
	Expect(seen.Get("developer-token")).To(Equal(""))
}

func TestClient_OmitsLoginCustomerIdWhenUnset(t *testing.T) {
	RegisterTestingT(t)

	var seen http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Clone()
		fmt.Fprint(w, `{"results":[]}`)
	}))
	defer server.Close()

	original := BaseURL
	BaseURL = server.URL
	defer func() { BaseURL = original }()

	_, err := NewClient("tok", "").Search(nil, "1234567890", "SELECT customer.id FROM customer")
	Expect(err).To(BeNil())
	_, present := seen["Login-Customer-Id"]
	Expect(present).To(BeFalse())
}

// partial_failure returns HTTP 200 with the failures inside the body: the
// successful operations HAVE been committed, so this must not be reported as if
// nothing happened.
func TestMutate_SurfacesPartialFailure(t *testing.T) {
	RegisterTestingT(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.URL.Path).To(Equal("/customers/1234567890/adGroupCriteria:mutate"))
		fmt.Fprint(w, `{
		  "results":[{"resourceName":"customers/1/adGroupCriteria/2~3"},{}],
		  "partialFailureError":{"code":3,"message":"Some failed","details":[{"errors":[
		    {"errorCode":{"criterionError":"KEYWORD_TEXT_TOO_LONG"},"message":"Too long.",
		     "location":{"fieldPathElements":[{"fieldName":"operations","index":1}]}}
		  ]}]}
		}`)
	}))
	defer server.Close()

	original := BaseURL
	BaseURL = server.URL
	defer func() { BaseURL = original }()

	resp, err := NewClient("tok", "").Mutate(nil, "1234567890", "adGroupCriteria",
		[]interface{}{map[string]interface{}{}, map[string]interface{}{}},
		MutateOptions{PartialFailure: true})

	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("operation 1"))
	Expect(err.Error()).To(ContainSubstring("KEYWORD_TEXT_TOO_LONG"))
	Expect(err.Error()).To(ContainSubstring("the rest were applied"))

	// The response must still come back so the caller can report what DID land.
	Expect(resp).ToNot(BeNil())
	Expect(MutatedResourceNames(resp)).To(Equal([]string{"customers/1/adGroupCriteria/2~3", ""}))
}

// googleAds:mutate nests each result under a per-type key rather than returning
// it flat, so both shapes have to be read.
func TestMutatedResourceNames_HandlesFlatAndNestedResults(t *testing.T) {
	RegisterTestingT(t)

	flat := map[string]interface{}{"results": []interface{}{
		map[string]interface{}{"resourceName": "customers/1/campaigns/2"},
	}}
	Expect(MutatedResourceNames(flat)).To(Equal([]string{"customers/1/campaigns/2"}))

	nested := map[string]interface{}{"results": []interface{}{
		map[string]interface{}{"campaignBudgetResult": map[string]interface{}{"resourceName": "customers/1/campaignBudgets/9"}},
		map[string]interface{}{"campaignResult": map[string]interface{}{"resourceName": "customers/1/campaigns/2"}},
	}}
	Expect(MutatedResourceNames(nested)).To(Equal([]string{
		"customers/1/campaignBudgets/9",
		"customers/1/campaigns/2",
	}))
}

func TestResourceID(t *testing.T) {
	RegisterTestingT(t)

	Expect(ResourceID("customers/1/campaigns/2")).To(Equal("2"))
	// A criterion's compound trailing segment IS its identity and must survive.
	Expect(ResourceID("customers/1/adGroupCriteria/2~3")).To(Equal("2~3"))
	Expect(ResourceID("")).To(Equal(""))
}

func TestListAccessibleCustomers_StripsThePrefix(t *testing.T) {
	RegisterTestingT(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.Method).To(Equal(http.MethodGet))
		Expect(r.URL.Path).To(Equal("/customers:listAccessibleCustomers"))
		fmt.Fprint(w, `{"resourceNames":["customers/1234567890","customers/9876543210"]}`)
	}))
	defer server.Close()

	original := BaseURL
	BaseURL = server.URL
	defer func() { BaseURL = original }()

	ids, err := NewClient("tok", "").ListAccessibleCustomers(nil)
	Expect(err).To(BeNil())
	Expect(ids).To(Equal([]string{"1234567890", "9876543210"}))
}

// --- result shapers ---

// An agent given "Found 12 campaigns" and nothing else has to make a second
// call to do anything useful, so the data goes IN tool_result (executor!305).
func TestRowsResult_EmbedsTheDataInToolResult(t *testing.T) {
	RegisterTestingT(t)

	out := RowsResult([]map[string]interface{}{
		{"campaign": map[string]interface{}{"name": "Brand"}, "metrics": map[string]interface{}{"costMicros": "5430000"}},
	}, "Found 1 campaign", nil)

	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(1))

	text, _ := out["tool_result"].(string)
	Expect(text).To(ContainSubstring("Found 1 campaign"))
	Expect(text).To(ContainSubstring("Brand"))
	// The converted money must be what the agent reads, so it cannot report
	// spend of 5,430,000.
	Expect(text).To(ContainSubstring(`"metrics.cost":"5.43"`))
}

// "Validated" and "Created" must never be mistakable for each other by a person
// skim-reading an execution or an agent deciding whether to move on.
func TestMutateResult_MarksADryRunUnmistakably(t *testing.T) {
	RegisterTestingT(t)

	resp := map[string]interface{}{"results": []interface{}{}}
	out := MutateResult(resp, MutateOptions{ValidateOnly: true}, "Created campaign \"Brand\"", nil)

	Expect(out["dry_run"]).To(Equal(true))
	Expect(out["tool_result"]).To(ContainSubstring("NOTHING was applied"))

	live := MutateResult(map[string]interface{}{"results": []interface{}{
		map[string]interface{}{"resourceName": "customers/1/campaigns/2"},
	}}, MutateOptions{}, "Created campaign", nil)

	Expect(live["dry_run"]).To(Equal(false))
	Expect(live["id"]).To(Equal("2"))
	Expect(live["tool_result"]).ToNot(ContainSubstring("NOTHING"))
}

func TestErrorResult(t *testing.T) {
	RegisterTestingT(t)

	out := ErrorResult("a customer ID is required")
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(Equal("a customer ID is required"))
	Expect(out["tool_result"]).To(ContainSubstring("Error: a customer ID is required"))
}
