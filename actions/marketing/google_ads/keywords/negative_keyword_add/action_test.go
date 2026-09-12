package negative_keyword_add

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

// Overrides REPLACE rather than append: core.FindConnection returns the first
// match, so an appended override would silently do nothing.
func inputs(extra ...[2]string) []*core.Connection {
	values := map[string]string{
		"credential":  "ya29.token",
		"customer_id": "123-456-7890",
		"level":       "CAMPAIGN",
		"campaign_id": "555",
		"keywords":    "free stuff\ncheap knockoff",
		"match_type":  "PHRASE",
	}
	order := []string{"credential", "customer_id", "level", "campaign_id", "ad_group_id", "keywords", "match_type", "validate_only", "partial_failure"}

	pairs := make([][2]string, 0, len(order))
	for _, e := range extra {
		values[e[0]] = e[1]
	}
	for _, name := range order {
		if values[name] == "" {
			continue
		}
		pairs = append(pairs, [2]string{name, values[name]})
	}
	return ins(pairs...)
}

func TestMain(m *testing.M) {
	// Unroutable by default, so an unstubbed test fails rather than calling the
	// live Google Ads API.
	gads.BaseURL = "http://127.0.0.1:1"
	os.Exit(m.Run())
}

func capture(t *testing.T, wantPath string, body *map[string]interface{}, response string) func() {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		RegisterTestingT(t)
		Expect(r.URL.Path).To(Equal(wantPath))
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

const twoCreated = `{"results":[
  {"resourceName":"customers/1234567890/campaignCriteria/555~1"},
  {"resourceName":"customers/1234567890/campaignCriteria/555~2"}
]}`

// The single most consequential field in this action. Without `negative: true`
// Google creates POSITIVE keywords, so an account that was trying to stop
// paying for these terms starts bidding on them instead. It fails silently —
// the call succeeds and the flow reports success.
func TestExecute_SetsNegativeTrue(t *testing.T) {
	RegisterTestingT(t)

	var body map[string]interface{}
	defer capture(t, "/customers/1234567890/campaignCriteria:mutate", &body, twoCreated)()

	out, err := Execute(nil, nil, inputs())
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))

	ops, _ := body["operations"].([]interface{})
	Expect(ops).To(HaveLen(2))

	for _, op := range ops {
		criterion, _ := op.(map[string]interface{})["create"].(map[string]interface{})
		Expect(criterion["negative"]).To(Equal(true), "omitting this creates a keyword the account BIDS on")
		Expect(criterion["campaign"]).To(Equal("customers/1234567890/campaigns/555"))

		keyword, _ := criterion["keyword"].(map[string]interface{})
		Expect(keyword["matchType"]).To(Equal("PHRASE"))
	}
}

// Campaign-level and ad-group-level negatives live in DIFFERENT resources with
// different parent fields; posting one to the other's endpoint is rejected.
func TestExecute_RoutesToTheRightResourcePerLevel(t *testing.T) {
	RegisterTestingT(t)

	var body map[string]interface{}
	defer capture(t, "/customers/1234567890/adGroupCriteria:mutate", &body, twoCreated)()

	_, err := Execute(nil, nil, inputs(
		[2]string{"level", "AD_GROUP"},
		[2]string{"campaign_id", ""},
		[2]string{"ad_group_id", "888"},
	))
	Expect(err).To(BeNil())

	ops, _ := body["operations"].([]interface{})
	criterion, _ := ops[0].(map[string]interface{})["create"].(map[string]interface{})
	Expect(criterion["adGroup"]).To(Equal("customers/1234567890/adGroups/888"))
	Expect(criterion).ToNot(HaveKey("campaign"))
	Expect(criterion["negative"]).To(Equal(true))
}

// Google Ads writes phrase match as "term" and exact as [term], and that is
// what people paste out of the UI or a report. Left in place, the brackets
// become part of the keyword text and block nothing.
func TestExecute_StripsMatchTypePunctuationFromPastedTerms(t *testing.T) {
	RegisterTestingT(t)

	var body map[string]interface{}
	defer capture(t, "/customers/1234567890/campaignCriteria:mutate", &body, twoCreated)()

	_, err := Execute(nil, nil, inputs([2]string{"keywords", "[free stuff]\n\"cheap knockoff\""}))
	Expect(err).To(BeNil())

	ops, _ := body["operations"].([]interface{})
	first, _ := ops[0].(map[string]interface{})["create"].(map[string]interface{})
	keyword, _ := first["keyword"].(map[string]interface{})
	Expect(keyword["text"]).To(Equal("free stuff"))
}

// A bulk add defaults to partial failure: one duplicate term should not throw
// away the other forty-nine.
func TestExecute_DefaultsToPartialFailure(t *testing.T) {
	RegisterTestingT(t)

	var body map[string]interface{}
	defer capture(t, "/customers/1234567890/campaignCriteria:mutate", &body, twoCreated)()

	_, err := Execute(nil, nil, inputs())
	Expect(err).To(BeNil())
	Expect(body["partialFailure"]).To(Equal(true))
}

// When some operations fail, the successful ones HAVE been committed. Reporting
// that as a flat failure would have someone re-run it and double-add the rest.
func TestExecute_ReportsWhatLandedOnPartialFailure(t *testing.T) {
	RegisterTestingT(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
		  "results":[{"resourceName":"customers/1234567890/campaignCriteria/555~1"},{}],
		  "partialFailureError":{"message":"Some failed","details":[{"errors":[
		    {"errorCode":{"criterionError":"DUPLICATE_KEYWORD"},"message":"Already excluded.",
		     "location":{"fieldPathElements":[{"fieldName":"operations","index":1}]}}
		  ]}]}
		}`)
	}))
	defer server.Close()

	original := gads.BaseURL
	gads.BaseURL = server.URL
	defer func() { gads.BaseURL = original }()

	out, err := Execute(nil, nil, inputs())
	Expect(err).To(BeNil())

	Expect(out["success"]).To(Equal(false))
	Expect(out["count"]).To(Equal(1), "one term WAS excluded")
	Expect(out["requested"]).To(Equal(2))
	Expect(out["tool_result"]).To(ContainSubstring("Excluded 1 of 2"))
	Expect(out["tool_result"]).To(ContainSubstring("DUPLICATE_KEYWORD"))
}

func TestExecute_ValidatesBeforeCalling(t *testing.T) {
	RegisterTestingT(t)

	for _, c := range []struct {
		name    string
		inputs  []*core.Connection
		wantMsg string
	}{
		{"no level", inputs([2]string{"level", ""}), "campaign or ad group level"},
		{"campaign level without an id", inputs([2]string{"campaign_id", ""}), "campaign ID is required"},
		{"ad group level without an id", inputs([2]string{"level", "AD_GROUP"}, [2]string{"campaign_id", ""}), "ad group ID is required"},
		{"no match type", inputs([2]string{"match_type", ""}), "match type is required"},
		{"bad match type", inputs([2]string{"match_type", "SORT_OF"}), "EXACT, PHRASE or BROAD"},
		{"no keywords", inputs([2]string{"keywords", "  "}), "one per line"},
	} {
		out, err := Execute(nil, nil, c.inputs)
		Expect(err).To(BeNil(), c.name)
		Expect(out["success"]).To(Equal(false), c.name)
		Expect(out["error"]).To(ContainSubstring(c.wantMsg), c.name)
	}
}
