package account_list

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	meta "flomation.app/automate/executor/actions/marketing/meta_ads"
	. "github.com/onsi/gomega"
)

func in(pairs ...[2]string) []*core.Connection {
	out := make([]*core.Connection, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, &core.Connection{Name: p[0], Type: core.ConnectionTypeString, Value: p[1]})
	}
	return out
}

// stub records the path Graph was asked for and serves the supplied accounts.
func stub(t *testing.T, accounts []map[string]interface{}) *string {
	t.Helper()
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": accounts})
	}))
	old := meta.BaseURL
	meta.BaseURL = srv.URL
	t.Cleanup(func() { meta.BaseURL = old; srv.Close() })
	return &gotPath
}

// The failure this exists for: a System User token with no ad account assigned
// returns an empty list and a 200. Reported as bare "Found 0 ad accounts", that
// is indistinguishable from a broken integration, and it sends the reader
// looking at scopes and tokens — the two things that are usually fine.
func TestExecute_ExplainsAnEmptyResult(t *testing.T) {
	RegisterTestingT(t)
	stub(t, nil)

	out, err := Execute(nil, nil, in([2]string{"access_token", "EAAG"}))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true), "an empty list is a valid answer, not a failure")
	Expect(out["count"]).To(Equal(0))

	summary, _ := out["tool_result"].(string)
	Expect(summary).To(ContainSubstring("Found 0 ad account"))
	Expect(summary).To(ContainSubstring("ASSIGNMENT problem"))
	Expect(summary).To(ContainSubstring("Assign Assets"))
	// The permission-level trap is the one that looks configured and is not.
	Expect(summary).To(ContainSubstring("Manage campaigns"))
	Expect(summary).To(ContainSubstring("Business ID"))
}

// With a business id the question changes from "what am I assigned" to "what
// exists", so the explanation must change with it — telling someone to assign
// an asset that does not exist wastes their time.
func TestExecute_EmptyExplanationDiffersForBusiness(t *testing.T) {
	RegisterTestingT(t)
	path := stub(t, nil)

	out, err := Execute(nil, nil, in(
		[2]string{"access_token", "EAAG"},
		[2]string{"business_id", "1507486194117794"},
	))

	Expect(err).To(BeNil())
	Expect(*path).To(Equal("/1507486194117794/owned_ad_accounts"))

	summary, _ := out["tool_result"].(string)
	Expect(summary).To(ContainSubstring("owns no ad accounts"))
	Expect(summary).To(ContainSubstring("Ads Manager"))
	Expect(summary).ToNot(ContainSubstring("ASSIGNMENT problem"), "the assignment advice is wrong for a business query")
}

func TestExecute_UsesMeEdgeByDefault(t *testing.T) {
	RegisterTestingT(t)
	path := stub(t, []map[string]interface{}{
		{"id": "act_1801854727675115", "name": "Flomation Ad Account", "currency": "GBP", "account_status": float64(1)},
	})

	out, err := Execute(nil, nil, in([2]string{"access_token", "EAAG"}))

	Expect(err).To(BeNil())
	Expect(*path).To(Equal("/me/adaccounts"))
	Expect(out["count"]).To(Equal(1))

	// A populated result must NOT carry the empty-case advice.
	summary, _ := out["tool_result"].(string)
	Expect(summary).To(ContainSubstring("Found 1 ad account"))
	Expect(summary).ToNot(ContainSubstring("ASSIGNMENT problem"))
	// The records are embedded so an AI caller gets the currency and status
	// without a second call.
	Expect(summary).To(ContainSubstring("GBP"))
}

func TestEmptyExplanation_NamesTheRightFix(t *testing.T) {
	RegisterTestingT(t)

	assigned := EmptyExplanation("")
	Expect(assigned).To(ContainSubstring("/me/adaccounts returns only"))
	Expect(assigned).To(ContainSubstring("System User"))

	owned := EmptyExplanation("123")
	Expect(owned).To(ContainSubstring("client accounts are separate lists"))
}
