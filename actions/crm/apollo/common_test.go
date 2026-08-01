package apollo_common

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestObjectResult(t *testing.T) {
	RegisterTestingT(t)

	obj := map[string]interface{}{"id": "p_123", "name": "Ada Lovelace", "email": "ada@x.com"}
	res := ObjectResult("", obj, "Enriched Ada Lovelace")

	Expect(res["success"]).To(BeTrue())
	Expect(res["error"]).To(Equal(""))
	Expect(res["id"]).To(Equal("p_123")) // derived from the object when id arg empty
	Expect(res["result"]).To(Equal(obj))
	// tool_result must carry the summary AND the object fields — an AI caller
	// only sees tool_result, so summary-only would starve it of the data.
	tr := res["tool_result"].(string)
	Expect(tr).To(HavePrefix("Enriched Ada Lovelace"))
	Expect(tr).To(ContainSubstring("Ada Lovelace"))
	Expect(tr).To(ContainSubstring("ada@x.com"))
}

func TestListResult(t *testing.T) {
	RegisterTestingT(t)

	items := []map[string]interface{}{{"name": "Ada", "email": "ada@x.com"}, {"name": "Grace"}}
	res := ListResult(items, "Found 2 people")
	Expect(res["success"]).To(BeTrue())
	Expect(res["count"]).To(Equal(2))
	Expect(res["results"]).To(Equal(items))
	// Regression: tool_result must embed the actual records, not just the count,
	// or an AI caller sees "Found 2 people" with no names/emails to act on.
	tr := res["tool_result"].(string)
	Expect(tr).To(HavePrefix("Found 2 people"))
	Expect(tr).To(ContainSubstring("Ada"))
	Expect(tr).To(ContainSubstring("ada@x.com"))
	Expect(tr).To(ContainSubstring("Grace"))

	// nil items normalise to an empty slice, not a null, so downstream nodes can
	// iterate safely; tool_result falls back to the bare summary (no empty array).
	res = ListResult(nil, "Found 0 people")
	Expect(res["count"]).To(Equal(0))
	Expect(res["results"]).To(Equal([]map[string]interface{}{}))
	Expect(res["tool_result"]).To(Equal("Found 0 people"))
}

func TestErrorResult(t *testing.T) {
	RegisterTestingT(t)

	res := ErrorResult("invalid parameter")
	Expect(res["success"]).To(BeFalse())
	Expect(res["error"]).To(Equal("invalid parameter"))
	Expect(res["tool_result"]).To(Equal("invalid parameter"))
}

func TestAPIErrorMessage(t *testing.T) {
	RegisterTestingT(t)

	// {"error":"…"}
	Expect((&APIError{Status: 422, Body: []byte(`{"error":"bad domain"}`)}).Message()).To(Equal("bad domain"))
	// {"error_message":"…"}
	Expect((&APIError{Status: 422, Body: []byte(`{"error_message":"nope"}`)}).Message()).To(Equal("nope"))
	// {"errors":["a","b"]}
	Expect((&APIError{Status: 422, Body: []byte(`{"errors":["a","b"]}`)}).Message()).To(Equal("a; b"))
	// unparseable body falls back to status + raw text
	Expect((&APIError{Status: 500, Body: []byte("upstream boom")}).Message()).To(ContainSubstring("HTTP 500"))
	// empty body → status only
	Expect((&APIError{Status: 429, Body: nil}).Message()).To(ContainSubstring("HTTP 429"))
}

func TestMapError(t *testing.T) {
	RegisterTestingT(t)

	res := MapError(&APIError{Status: 422, Body: []byte(`{"error":"rate limited"}`)})
	Expect(res["success"]).To(BeFalse())
	Expect(res["error"]).To(Equal("rate limited"))

	res = MapError(errors.New("connection reset"))
	Expect(res["error"]).To(Equal("connection reset"))
}

func TestGetAPIKey(t *testing.T) {
	RegisterTestingT(t)

	// Happy path.
	key, err := GetAPIKey([]*core.Connection{{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "abc123"}})
	Expect(err).ToNot(HaveOccurred())
	Expect(key).To(Equal("abc123"))

	// Missing.
	_, err = GetAPIKey([]*core.Connection{})
	Expect(err).To(HaveOccurred())

	// An unresolved ${secrets.X} reference is rejected with a helpful message
	// rather than being sent to Apollo as a literal.
	_, err = GetAPIKey([]*core.Connection{{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "${secrets.Missing}"}})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("did not resolve"))
}

func TestStringList(t *testing.T) {
	RegisterTestingT(t)

	inputs := []*core.Connection{{Name: "ids", Type: core.ConnectionTypeString, Value: " a , b ,, c "}}
	Expect(StringList("ids", inputs)).To(Equal([]string{"a", "b", "c"}))
	Expect(StringList("missing", inputs)).To(BeNil())
	Expect(StringList("ids", []*core.Connection{{Name: "ids", Value: "   "}})).To(BeNil())
}

// TestRangeList guards the headcount-range bug: a range like "50,5000" must stay
// ONE element (Apollo wants ["50,5000"], not ["50","5000"]); multiple ranges are
// separated by ';' or newlines, never by the comma inside a range.
func TestRangeList(t *testing.T) {
	RegisterTestingT(t)

	// Single range keeps its inner comma.
	Expect(RangeList("r", []*core.Connection{{Name: "r", Value: "50,5000"}})).To(Equal([]string{"50,5000"}))
	// Multiple ranges split on ';' (and tolerate whitespace + blanks).
	Expect(RangeList("r", []*core.Connection{{Name: "r", Value: " 1,10 ; 11,50 ;; 51,5000 "}})).
		To(Equal([]string{"1,10", "11,50", "51,5000"}))
	// Newline-separated ranges also work.
	Expect(RangeList("r", []*core.Connection{{Name: "r", Value: "1,10\n11,50"}})).To(Equal([]string{"1,10", "11,50"}))
	// Absent / blank → nil (field omitted).
	Expect(RangeList("missing", nil)).To(BeNil())
	Expect(RangeList("r", []*core.Connection{{Name: "r", Value: "  "}})).To(BeNil())
}

func TestSetHelpers(t *testing.T) {
	RegisterTestingT(t)

	inputs := []*core.Connection{
		{Name: "s", Type: core.ConnectionTypeString, Value: "hello"},
		{Name: "empty", Type: core.ConnectionTypeString, Value: ""},
		{Name: "n", Type: core.ConnectionTypeInteger, Value: int64(25)},
		{Name: "b", Type: core.ConnectionTypeBoolean, Value: true},
		{Name: "list", Type: core.ConnectionTypeString, Value: "x,y"},
		{Name: "amt", Type: core.ConnectionTypeString, Value: "£1,234.50"},
	}
	body := map[string]interface{}{}
	SetString(body, "s", "s", inputs)
	SetString(body, "empty", "empty", inputs) // blank omitted
	SetInt(body, "n", "n", inputs)
	SetBool(body, "b", "b", inputs)
	SetList(body, "list", "list", inputs)
	SetNumberValue(body, "amt", "amt", inputs)

	Expect(body["s"]).To(Equal("hello"))
	Expect(body).ToNot(HaveKey("empty"))
	Expect(body["n"]).To(Equal(int64(25)))
	Expect(body["b"]).To(Equal(true))
	Expect(body["list"]).To(Equal([]string{"x", "y"}))
	Expect(body["amt"]).To(Equal(1234.50))
}

func TestObjArr(t *testing.T) {
	RegisterTestingT(t)

	resp := map[string]interface{}{
		"person": map[string]interface{}{"id": "p1"},
		"people": []interface{}{map[string]interface{}{"id": "a"}, "skip", map[string]interface{}{"id": "b"}},
	}
	Expect(IDOf(Obj(resp, "person"))).To(Equal("p1"))
	arr := Arr(resp, "people")
	Expect(arr).To(HaveLen(2)) // the non-object element is skipped
	Expect(IDOf(arr[1])).To(Equal("b"))
	Expect(Obj(resp, "missing")).To(BeNil())
}

// TestClientRequest exercises the real HTTP path against a mock server: the
// X-Api-Key header must be sent, the JSON body round-tripped, and the response
// decoded.
func TestClientRequest(t *testing.T) {
	RegisterTestingT(t)

	var gotKey, gotMethod, gotPath, gotQuery string
	var gotBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-Api-Key")
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotQuery = r.URL.Query().Get("domain")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"person":{"id":"p_9","name":"Ada"}}`))
	}))
	defer srv.Close()

	orig := BaseURL
	BaseURL = srv.URL
	defer func() { BaseURL = orig }()

	c := NewClient("secret-key")
	resp, err := c.Post(nil, "/people/match", map[string]interface{}{"email": "a@b.com"})
	Expect(err).ToNot(HaveOccurred())
	Expect(gotKey).To(Equal("secret-key"))
	Expect(gotMethod).To(Equal("POST"))
	Expect(gotPath).To(Equal("/people/match"))
	Expect(gotBody).To(HaveKeyWithValue("email", "a@b.com"))
	Expect(IDOf(Obj(resp, "person"))).To(Equal("p_9"))

	// A GET with url.Values is query-encoded onto the URL.
	q := url.Values{}
	q.Set("domain", "example.com")
	_, err = c.Get(nil, "/organizations/enrich", q)
	Expect(err).ToNot(HaveOccurred())
	Expect(gotMethod).To(Equal("GET"))
	Expect(gotQuery).To(Equal("example.com"))
}

// TestClientRequestError confirms a non-2xx response becomes an *APIError
// carrying the parsed Apollo message (so actions can MapError into a graceful
// success=false).
func TestClientRequestError(t *testing.T) {
	RegisterTestingT(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"error":"invalid domain"}`))
	}))
	defer srv.Close()

	orig := BaseURL
	BaseURL = srv.URL
	defer func() { BaseURL = orig }()

	_, err := NewClient("k").Get(nil, "/organizations/enrich", nil)
	Expect(err).To(HaveOccurred())
	var ae *APIError
	Expect(errors.As(err, &ae)).To(BeTrue())
	Expect(ae.Status).To(Equal(422))
	Expect(ae.Message()).To(Equal("invalid domain"))
}
